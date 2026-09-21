package service

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
)

type ScheduleService struct {
	repo        *repository.ScheduleRepository
	staffRepo   *repository.StaffRepository
	shiftRepo   *repository.ShiftRepository
	ruleRepo    *repository.ScheduleRuleRepository
	holidayRepo *repository.HolidayRepository
	engine      *RuleEngine
	logger      *slog.Logger
}

func NewScheduleService(r *repository.ScheduleRepository, sr *repository.StaffRepository, sh *repository.ShiftRepository, rr *repository.ScheduleRuleRepository, hr *repository.HolidayRepository, eng *RuleEngine, l *slog.Logger) *ScheduleService {
	return &ScheduleService{r, sr, sh, rr, hr, eng, l}
}

func (s *ScheduleService) List(dept, staff uint, from, to time.Time) ([]model.Schedule, error) {
	v, e := s.repo.List(dept, staff, from, to)
	if e != nil {
		return nil, fmt.Errorf("list schedules: %w", e)
	}
	return v, nil
}

type genState struct {
	streak       int
	workedNight  bool
	holidayRests int
}

// Generate creates schedules for [from,to] while obeying the department's
// current rules: maximum consecutive working days, no day shift after a night
// shift, and holiday rest priority. Every produced row is re-checked against
// the rules and carries its conflict_reason so conflicts are visible in the
// schedule view even after filtering or refresh.
func (s *ScheduleService) Generate(dept uint, from, to time.Time) (int, error) {
	from, to = truncateDay(from), truncateDay(to)
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
	cfg := s.engine.config(dept)
	seedFrom := from.AddDate(0, 0, -(cfg.maxConsecutive + 2))

	holidays, e := s.holidayRepo.List()
	if e != nil {
		return 0, fmt.Errorf("list holidays: %w", e)
	}
	holidaySet := map[string]bool{}
	seedHolidaySet := map[string]bool{}
	for _, h := range holidays {
		if !h.IsWorkday && !h.Date.Before(from) && !h.Date.After(to) {
			holidaySet[dayKey(h.Date)] = true
		}
		if !h.IsWorkday && !h.Date.Before(seedFrom) && !h.Date.After(to) {
			seedHolidaySet[dayKey(h.Date)] = true
		}
	}

	// Seed state from existing rows before the range so boundary days are
	// still compliant with the consecutive-work and night-to-day rules.
	state := map[uint]*genState{}
	for _, st := range staffs {
		state[st.ID] = &genState{}
	}
	prior, err := s.repo.List(dept, 0, seedFrom, from.AddDate(0, 0, -1))
	if err != nil {
		return 0, fmt.Errorf("load prior schedules: %w", err)
	}
	sort.Slice(prior, func(i, j int) bool { return prior[i].WorkDate.Before(prior[j].WorkDate) })
	for _, p := range prior {
		st := state[p.StaffID]
		if st == nil {
			continue
		}
		if p.Shift.Kind == model.ShiftRest {
			st.streak = 0
			st.workedNight = false
		} else {
			st.streak++
			st.workedNight = p.Shift.Kind == model.ShiftNight
		}
		if seedHolidaySet[dayKey(p.WorkDate)] && p.Shift.Kind == model.ShiftRest {
			st.holidayRests++
		}
	}

	if e = s.repo.DeleteRange(dept, from, to); e != nil {
		return 0, fmt.Errorf("clear range: %w", e)
	}

	created := 0
	for d, i := from, 0; !d.After(to); d, i = d.AddDate(0, 0, 1), i+1 {
		isHoliday := holidaySet[dayKey(d)]
		resting := map[uint]bool{}
		nightStaff := uint(0)

		// Forced rests: consecutive-work cap and night-to-day restriction.
		for _, st := range staffs {
			g := state[st.ID]
			if g.streak >= cfg.maxConsecutive || (cfg.nightToDay && g.workedNight) {
				resting[st.ID] = true
			}
		}
		// Holiday priority: when nobody is already resting on the holiday,
		// the member with the fewest holiday rests takes the rest day.
		if cfg.holidayPriority && isHoliday && len(resting) == 0 && len(staffs) > 1 {
			pick := -1
			for idx, st := range staffs {
				if pick == -1 || state[st.ID].holidayRests < state[staffs[pick].ID].holidayRests {
					pick = idx
				}
			}
			if pick >= 0 {
				resting[staffs[pick].ID] = true
			}
		}
		// Rotating night duty; anyone resting today is skipped.
		for idx, st := range staffs {
			if resting[st.ID] {
				continue
			}
			if (i+idx)%5 == 0 {
				nightStaff = st.ID
				break
			}
		}
		// Minimum coverage: a department cannot have nobody on duty. When
		// every member is resting and no night duty was assigned, one is
		// pulled back to work. The rule violation this inevitably creates is
		// recorded on the member's row by the post-generation rule re-check.
		forceDay := uint(0)
		if len(staffs) > 0 && len(resting) == len(staffs) && nightStaff == 0 {
			forceDay = staffs[0].ID
			resting[forceDay] = false
		}

		for _, st := range staffs {
			g := state[st.ID]
			shift := day
			switch {
			case st.ID == forceDay:
				shift = day
				g.streak++
			case resting[st.ID]:
				shift = rest
				g.streak = 0
				if isHoliday {
					g.holidayRests++
				}
			case st.ID == nightStaff:
				shift = night
				g.streak++
			default:
				g.streak++
			}
			g.workedNight = shift.Kind == model.ShiftNight
			sc := model.Schedule{DepartmentID: dept, StaffID: st.ID, ShiftID: shift.ID, WorkDate: d}
			if e = s.repo.Create(&sc); e != nil {
				return created, fmt.Errorf("create schedule: %w", e)
			}
			created++
		}
	}

	if err := s.refreshReasons(dept, from, to); err != nil {
		return created, err
	}
	return created, nil
}

// refreshReasons re-checks [from,to] against the latest rules and persists
// conflict reasons onto the rows, including the day after the range so a
// night shift on the last day is fully accounted for.
func (s *ScheduleService) refreshReasons(dept uint, from, to time.Time) error {
	windowFrom, windowTo := from.AddDate(0, 0, -1), to.AddDate(0, 0, 1)
	reasons, err := s.engine.Check(dept, windowFrom, windowTo, nil)
	if err != nil {
		return err
	}
	rows, err := s.repo.List(dept, 0, windowFrom, windowTo)
	if err != nil {
		return fmt.Errorf("load schedules for conflict write-back: %w", err)
	}
	reasonByID := map[uint]string{}
	for _, r := range rows {
		reasonByID[r.ID] = ""
	}
	for id, reason := range reasons {
		if _, ok := reasonByID[id]; ok {
			reasonByID[id] = reason
		}
	}
	for id, reason := range reasonByID {
		if e := s.repo.UpdateConflictReason(id, reason); e != nil {
			return fmt.Errorf("persist conflict reason: %w", e)
		}
	}
	return nil
}

// Update applies a manual adjustment. The edit is simulated against the
// latest rules first; when it introduces any conflict the whole operation is
// rejected and the original schedule stays untouched.
func (s *ScheduleService) Update(id, shiftID uint, note string) error {
	v, e := s.repo.Find(id)
	if e != nil {
		return fmt.Errorf("find schedule: %w", e)
	}
	target, e := s.shiftRepo.ByID(shiftID)
	if e != nil {
		return fmt.Errorf("find target shift: %w", e)
	}

	pad := s.engine.config(v.DepartmentID).maxConsecutive + 1
	day0 := truncateDay(v.WorkDate)
	windowFrom := day0.AddDate(0, 0, -pad)
	windowTo := day0.AddDate(0, 0, pad)

	baseline, err := s.engine.Check(v.DepartmentID, windowFrom, windowTo, nil)
	if err != nil {
		return err
	}
	edited := v
	edited.ShiftID = shiftID
	edited.Shift = target
	simulated, err := s.engine.Check(v.DepartmentID, windowFrom, windowTo, []model.Schedule{edited})
	if err != nil {
		return err
	}
	if conflicts := newConflicts(v.DepartmentID, baseline, simulated, s.repo, windowFrom, windowTo); len(conflicts) > 0 {
		return &ScheduleConflictError{Conflicts: conflicts}
	}

	v.ShiftID = shiftID
	v.Note = note
	if e = s.repo.Save(&v); e != nil {
		return fmt.Errorf("update schedule: %w", e)
	}
	// Simulated state equals the committed state, so write its reasons back
	// without another query.
	if err := s.persistReasons(v.DepartmentID, windowFrom, windowTo, simulated); err != nil {
		return err
	}
	return nil
}

func (s *ScheduleService) persistReasons(dept uint, from, to time.Time, reasons map[uint]string) error {
	rows, err := s.repo.List(dept, 0, from, to)
	if err != nil {
		return fmt.Errorf("load schedules for conflict write-back: %w", err)
	}
	want := map[uint]string{}
	for _, r := range rows {
		want[r.ID] = reasons[r.ID]
	}
	for id, reason := range want {
		if e := s.repo.UpdateConflictReason(id, reason); e != nil {
			return fmt.Errorf("persist conflict reason: %w", e)
		}
	}
	return nil
}

// newConflicts returns violations present in simulated but absent in baseline,
// restricted to the tight window around the edited day.
func newConflicts(dept uint, baseline, simulated map[uint]string, repo *repository.ScheduleRepository, from, to time.Time) []ScheduleConflict {
	rows, err := repo.List(dept, 0, from, to)
	if err != nil {
		return nil
	}
	byID := map[uint]model.Schedule{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	out := make([]ScheduleConflict, 0)
	for id, reason := range simulated {
		if reason == "" || reason == baseline[id] {
			continue
		}
		r, ok := byID[id]
		if !ok {
			continue
		}
		for _, part := range splitReasons(reason) {
			if !containsReason(splitReasons(baseline[id]), part) {
				out = append(out, ScheduleConflict{ScheduleID: id, StaffID: r.StaffID, WorkDate: r.WorkDate, Reason: part})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].WorkDate.Equal(out[j].WorkDate) {
			return out[i].StaffID < out[j].StaffID
		}
		return out[i].WorkDate.Before(out[j].WorkDate)
	})
	return out
}

func splitReasons(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, "；")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func containsReason(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
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
