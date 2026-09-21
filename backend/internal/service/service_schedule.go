package service

import (
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"gorm.io/gorm"
)

type ScheduleService struct {
	db          *gorm.DB
	repo        *repository.ScheduleRepository
	staffRepo   *repository.StaffRepository
	shiftRepo   *repository.ShiftRepository
	ruleRepo    *repository.ScheduleRuleRepository
	holidayRepo *repository.HolidayRepository
	logger      *slog.Logger
}

func NewScheduleService(db *gorm.DB, r *repository.ScheduleRepository, sr *repository.StaffRepository, sh *repository.ShiftRepository, rr *repository.ScheduleRuleRepository, hr *repository.HolidayRepository, l *slog.Logger) *ScheduleService {
	return &ScheduleService{db: db, repo: r, staffRepo: sr, shiftRepo: sh, ruleRepo: rr, holidayRepo: hr, logger: l}
}

func (s *ScheduleService) List(dept, staff uint, from, to time.Time) ([]model.Schedule, error) {
	v, e := s.repo.List(dept, staff, from, to)
	if e != nil {
		return nil, fmt.Errorf("list schedules: %w", e)
	}
	return v, nil
}

// planningState 保存自动生成过程中每人的排班状态。
type planningState struct {
	consecutive int
	holidayWork int
}

const contextWindowDays = 14

// Generate 按最新规则生成班表：遵守连续工作上限、夜班后不得排白班、节假日优先；
// 无法完全避免的冲突会以 has_conflict/conflict_reasons 逐条记录在班表中。
func (s *ScheduleService) Generate(dept uint, from, to time.Time) (int, error) {
	from = normDate(from)
	to = normDate(to)
	if to.Before(from) || to.Sub(from) > 366*24*time.Hour {
		return 0, fmt.Errorf("invalid schedule range")
	}
	staffs, e := s.staffRepo.List(dept)
	if e != nil {
		return 0, fmt.Errorf("list scheduling staff: %w", e)
	}
	if len(staffs) == 0 {
		return 0, fmt.Errorf("department has no active staff")
	}
	day, e := s.shiftRepo.ByKind(model.ShiftDay)
	if e != nil {
		return 0, fmt.Errorf("get day shift: %w", e)
	}
	night, e := s.shiftRepo.ByKind(model.ShiftNight)
	if e != nil {
		return 0, fmt.Errorf("get night shift: %w", e)
	}
	rest, e := s.shiftRepo.ByKind(model.ShiftRest)
	if e != nil {
		return 0, fmt.Errorf("get rest shift: %w", e)
	}
	rule, e := s.ruleRepo.ByDepartment(dept)
	if e != nil {
		rule = model.ScheduleRule{DepartmentID: dept}
	}
	if rule.MaxConsecutiveDays <= 0 {
		rule.MaxConsecutiveDays = 5
	}
	holidays, e := s.holidayRepo.List()
	if e != nil {
		return 0, fmt.Errorf("list holidays: %w", e)
	}
	allShifts, e := s.shiftRepo.List()
	if e != nil {
		return 0, fmt.Errorf("list shifts: %w", e)
	}

	// 载入生成区间前后窗口内的班表：连续工作计数、夜班后白班与节假日轮班都要承接历史；
	// 区间末尾与既有班表的衔接冲突也要能在复核时发现并记录到本次生成行上。
	windowRows, e := s.repo.List(dept, 0, from.AddDate(0, 0, -contextWindowDays), to.AddDate(0, 0, contextWindowDays))
	if e != nil {
		return 0, fmt.Errorf("list surrounding schedules: %w", e)
	}
	planned := map[string]scheduleCell{}
	for _, row := range windowRows {
		planned[cellKey(row.WorkDate, row.StaffID)] = scheduleCell{id: row.ID, shiftID: row.ShiftID, kind: row.Shift.Kind}
	}

	state := map[uint]*planningState{}
	for _, st := range staffs {
		state[st.ID] = &planningState{
			consecutive: trailingWorkdays(st.ID, from, planned),
			holidayWork: priorHolidayWork(st.ID, from, holidays, planned),
		}
	}

	dayCount := 0
	holidayOrdinal := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dayCount++
		holiday := isHolidayDay(holidays, d)
		if holiday {
			holidayOrdinal++
		}
		nightIdx := (dayCount - 1) % len(staffs)
		// 节假日值班人在处理当天所有人之前一次性确定，避免循环中状态变化改变选择。
		holidayHolder := uint(0)
		if holiday {
			holidayHolder = holidayHolderID(staffs, state, holidayOrdinal)
		}
		for j, st := range staffs {
			ps := state[st.ID]
			prev := planned[cellKey(d.AddDate(0, 0, -1), st.ID)]
			var chosen model.Shift
			switch {
			// 硬性规则：夜班后不得接白班，次日强制休息（同时释放连续工作计数）。
			case rule.ForbidNightToDay && prev.kind == model.ShiftNight:
				chosen = rest
			case ps.consecutive >= rule.MaxConsecutiveDays:
				chosen = rest
			case holiday:
				chosen = pickHolidayShift(st, holidayHolder, day, rest, rule, d, state, planned)
			case j == nightIdx && canWork(ps, rule) && nightPlaceable(d, st.ID, planned):
				chosen = night
			default:
				chosen = day
			}
			planned[cellKey(d, st.ID)] = scheduleCell{shiftID: chosen.ID, kind: chosen.Kind}
			if chosen.Kind == model.ShiftRest {
				ps.consecutive = 0
			} else {
				ps.consecutive++
				if holiday {
					ps.holidayWork++
				}
			}
		}
	}

	// 以完整网格按最新规则复核，把无法回避的冲突记录到每人对应的排班行。
	ctx := newConflictContext(rule, holidays, allShifts, nil)
	for key, cell := range planned {
		ctx.cells[key] = cell
	}
	conflicts := map[string]ConflictReasons{}
	for key, cell := range planned {
		date, staffID := parseCellKey(key)
		if date.Before(from) {
			continue // 历史行不在本次生成范围内，不改写其冲突标记
		}
		if reasons := ctx.reasons(date, staffID, cell); len(reasons) > 0 {
			conflicts[key] = reasons
		}
	}

	created := 0
	e = s.db.Transaction(func(tx *gorm.DB) error {
		txRepo := s.repo.WithTx(tx)
		if err := txRepo.DeleteRange(dept, from, to); err != nil {
			return fmt.Errorf("clear range: %w", err)
		}
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			for _, st := range staffs {
				key := cellKey(d, st.ID)
				cell := planned[key]
				row := &model.Schedule{
					DepartmentID: dept,
					StaffID:      st.ID,
					ShiftID:      cell.shiftID,
					WorkDate:     d,
				}
				if reasons, ok := conflicts[key]; ok {
					row.HasConflict = true
					row.ConflictReasons = reasons.Error()
				}
				if err := txRepo.Create(row); err != nil {
					return fmt.Errorf("create schedule: %w", err)
				}
				created++
			}
		}
		return nil
	})
	if e != nil {
		return 0, e
	}
	return created, nil
}

// holidayHolderID 选择节假日值班人：累计节假日值班最少者优先，相同情况下按
// 节假日序号轮班（序号取模），保证节假日值班在科室成员间轮循。
func holidayHolderID(staffs []model.Staff, state map[uint]*planningState, holidayOrdinal int) uint {
	minWork := -1
	for _, st := range staffs {
		w := state[st.ID].holidayWork
		if minWork < 0 || w < minWork {
			minWork = w
		}
	}
	candidates := make([]model.Staff, 0, len(staffs))
	for _, st := range staffs {
		if state[st.ID].holidayWork == minWork {
			candidates = append(candidates, st)
		}
	}
	sort.Slice(candidates, func(a, b int) bool { return candidates[a].ID < candidates[b].ID })
	return candidates[(holidayOrdinal-1)%len(candidates)].ID
}

// pickHolidayShift 执行“节假日优先”：只有预先轮选出的值班人当天上白班，其余人休息；
// 值班人若触碰连续工作上限或夜班后白班，也安排休息（当天无人值班的冲突由复核记录）。
func pickHolidayShift(self model.Staff, holderID uint, day, rest model.Shift, rule model.ScheduleRule, d time.Time, state map[uint]*planningState, planned map[string]scheduleCell) model.Shift {
	if self.ID == holderID && canWork(state[self.ID], rule) && dayPlaceable(d, self.ID, planned) {
		return day
	}
	return rest
}

// Update 手动调整前按最新规则复核：该员工窗口内任何一天仍有冲突都整次拒绝，
// 原班表保持不变；复核通过则清除该员工窗口内遗留的冲突标记。
func (s *ScheduleService) Update(id, shiftID uint, note string) error {
	var conflictErr error
	e := s.db.Transaction(func(tx *gorm.DB) error {
		txRepo := s.repo.WithTx(tx)
		v, err := txRepo.Find(id)
		if err != nil {
			return fmt.Errorf("find schedule: %w", err)
		}
		from := v.WorkDate.AddDate(0, 0, -contextWindowDays)
		to := v.WorkDate.AddDate(0, 0, contextWindowDays)
		ctx, err := s.loadContext(tx, v.DepartmentID, from, to)
		if err != nil {
			return err
		}
		cell := scheduleCell{id: v.ID, shiftID: shiftID, kind: ctx.kindOf(shiftID)}
		if cell.kind == "" {
			return fmt.Errorf("shift %d not found", shiftID)
		}
		ctx.add(v.WorkDate, v.StaffID, cell)
		if reasons := joinReasons(ctx.staffConflicts(v.StaffID)); len(reasons) > 0 {
			conflictErr = AsConflictError(reasons)
			return conflictErr
		}
		v.ShiftID = shiftID
		v.Note = note
		if err = txRepo.Save(&v); err != nil {
			return fmt.Errorf("update schedule: %w", err)
		}
		if err = txRepo.ClearConflicts(v.DepartmentID, v.StaffID, normDate(from), normDate(to)); err != nil {
			return fmt.Errorf("clear conflict flags: %w", err)
		}
		return nil
	})
	if conflictErr != nil {
		return conflictErr
	}
	return e
}

// loadContext 在给定事务上装配最新规则、节假日、班次与网格窗口（全部走 tx，
// 避免单连接场景下事务内再取连接造成自锁）。
func (s *ScheduleService) loadContext(tx *gorm.DB, dept uint, from, to time.Time) (*conflictContext, error) {
	rule, err := s.ruleRepo.WithTx(tx).ByDepartment(dept)
	if err != nil {
		rule = model.ScheduleRule{DepartmentID: dept}
	}
	holidays, err := s.holidayRepo.WithTx(tx).List()
	if err != nil {
		return nil, fmt.Errorf("list holidays: %w", err)
	}
	shifts, err := s.shiftRepo.WithTx(tx).List()
	if err != nil {
		return nil, fmt.Errorf("list shifts: %w", err)
	}
	rows, err := s.repo.WithTx(tx).List(dept, 0, normDate(from), normDate(to))
	if err != nil {
		return nil, fmt.Errorf("list schedules for rule check: %w", err)
	}
	return newConflictContext(rule, holidays, shifts, rows), nil
}

// ValidateSwap 模拟交换两人的排班并按最新规则复核。
// 返回 ErrScheduleConflict 时整次调班审批必须拒绝，班表与申请状态保持不变。
func (s *ScheduleService) ValidateSwap(tx *gorm.DB, a, b model.Schedule) error {
	if a.DepartmentID != b.DepartmentID {
		return fmt.Errorf("schedules do not belong to the same department")
	}
	from := normDate(a.WorkDate)
	if normDate(b.WorkDate).Before(from) {
		from = normDate(b.WorkDate)
	}
	to := normDate(a.WorkDate)
	if normDate(b.WorkDate).After(to) {
		to = normDate(b.WorkDate)
	}
	ctx, err := s.loadContext(tx, a.DepartmentID, from.AddDate(0, 0, -contextWindowDays), to.AddDate(0, 0, contextWindowDays))
	if err != nil {
		return err
	}
	cellA := scheduleCell{id: a.ID, shiftID: a.ShiftID, kind: ctx.kindOf(a.ShiftID)}
	cellB := scheduleCell{id: b.ID, shiftID: b.ShiftID, kind: ctx.kindOf(b.ShiftID)}
	if cellA.kind == "" || cellB.kind == "" {
		return fmt.Errorf("shift kind missing for swap schedules")
	}
	// 覆盖后的两个格子同时参与判定，再分别复核两位员工窗口内的全部排班。
	ctx.add(a.WorkDate, b.StaffID, cellA)
	ctx.add(b.WorkDate, a.StaffID, cellB)
	reasons := joinReasons(ctx.staffConflicts(a.StaffID), ctx.staffConflicts(b.StaffID))
	return AsConflictError(reasons)
}

func (s *ScheduleService) Stats(dept uint, month time.Time) ([]map[string]any, error) {
	from := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.Local)
	to := from.AddDate(0, 1, -1)
	items, e := s.repo.List(dept, 0, from, to)
	if e != nil {
		return nil, fmt.Errorf("list statistics: %w", e)
	}
	m := map[uint]map[string]any{}
	for _, x := range items {
		a := m[x.StaffID]
		if a == nil {
			a = map[string]any{"staff_id": x.StaffID, "name": x.Staff.Name, "attendance_days": 0, "day_shifts": 0, "night_shifts": 0, "evening_shifts": 0, "overtime_hours": 0.0}
			m[x.StaffID] = a
		}
		if x.Shift.Kind != model.ShiftRest {
			a["attendance_days"] = a["attendance_days"].(int) + 1
		}
		k := string(x.Shift.Kind) + "_shifts"
		if _, ok := a[k]; ok {
			a[k] = a[k].(int) + 1
		}
		if x.Shift.Kind == model.ShiftNight {
			a["overtime_hours"] = a["overtime_hours"].(float64) + 4
		}
	}
	out := make([]map[string]any, 0, len(m))
	for _, x := range m {
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["staff_id"].(uint) < out[j]["staff_id"].(uint) })
	return out, nil
}

// canWork 表示该员工当前连续工作计数尚未触顶。
func canWork(ps *planningState, rule model.ScheduleRule) bool {
	return ps.consecutive < rule.MaxConsecutiveDays
}

// dayPlaceable 预判白班是否违反“夜班后不得排白班”。
func dayPlaceable(date time.Time, staffID uint, planned map[string]scheduleCell) bool {
	prev, ok := planned[cellKey(normDate(date).AddDate(0, 0, -1), staffID)]
	return !ok || prev.kind != model.ShiftNight
}

// nightPlaceable 预判夜班是否可行：前一日不能是夜班，次日已排白班也不行；
// 次日尚未规划时，生成流程会在次日对该员工强制休息。
func nightPlaceable(date time.Time, staffID uint, planned map[string]scheduleCell) bool {
	d := normDate(date)
	if prev, ok := planned[cellKey(d.AddDate(0, 0, -1), staffID)]; ok && prev.kind == model.ShiftNight {
		return false
	}
	if next, ok := planned[cellKey(d.AddDate(0, 0, 1), staffID)]; ok && next.kind != model.ShiftRest {
		return false
	}
	return true
}

// trailingWorkdays 统计生成起点之前连续工作（不含休息）的天数。
func trailingWorkdays(staffID uint, from time.Time, planned map[string]scheduleCell) int {
	n := 0
	for d := normDate(from).AddDate(0, 0, -1); ; d = d.AddDate(0, 0, -1) {
		cell, ok := planned[cellKey(d, staffID)]
		if !ok || cell.kind == model.ShiftRest {
			return n
		}
		n++
	}
}

// priorHolidayWork 统计生成起点前窗口内该员工在法定节假日的值班天数，用于节假日轮班。
func priorHolidayWork(staffID uint, from time.Time, holidays []model.Holiday, planned map[string]scheduleCell) int {
	n := 0
	for _, h := range holidays {
		if h.IsWorkday || !normDate(h.Date).Before(normDate(from)) {
			continue
		}
		if cell, ok := planned[cellKey(h.Date, staffID)]; ok && cell.kind != model.ShiftRest {
			n++
		}
	}
	return n
}

func isHolidayDay(holidays []model.Holiday, date time.Time) bool {
	_, ok := holidayOn(holidays, date)
	return ok
}

func holidayOn(holidays []model.Holiday, date time.Time) (model.Holiday, bool) {
	key := normDate(date).Format("2006-01-02")
	for _, h := range holidays {
		if !h.IsWorkday && normDate(h.Date).Format("2006-01-02") == key {
			return h, true
		}
	}
	return model.Holiday{}, false
}

func parseCellKey(key string) (time.Time, uint) {
	parts := strings.SplitN(key, "|", 2)
	d, _ := time.ParseInLocation("2006-01-02", parts[0], time.Local)
	id, _ := strconv.ParseUint(parts[1], 10, 64)
	return d, uint(id)
}
