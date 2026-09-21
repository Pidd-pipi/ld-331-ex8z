package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gbsched/hospital-scheduler/internal/model"
)

// ErrScheduleConflict 表示按最新排班规则复核未通过。冲突时整次操作必须拒绝，
// 调用方用 errors.Is 判断并保持原班表与申请状态不变。
var ErrScheduleConflict = errors.New("排班冲突")

// ConflictReasons 以可读中文描述单个排班单元命中的全部规则冲突。
type ConflictReasons []string

func (r ConflictReasons) Error() string { return strings.Join(r, "；") }

// AsConflictError 在存在冲突时返回带原因的 ErrScheduleConflict，否则返回 nil。
func AsConflictError(reasons ConflictReasons) error {
	if len(reasons) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrScheduleConflict, reasons.Error())
}

// scheduleCell 是冲突检测视角下的最小排班单元（不依赖数据库行）。
type scheduleCell struct {
	id      uint
	shiftID uint
	kind    model.ShiftKind
}

func normDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func cellKey(date time.Time, staffID uint) string {
	return normDate(date).Format("2006-01-02") + "|" + strconv.FormatUint(uint64(staffID), 10)
}

// conflictContext 保存一次规则复核所需的班表网格、规则与节假日。
// 自动生成、手动调整与调班审批共用同一套判定逻辑，确保“最新规则”口径一致。
type conflictContext struct {
	rule     model.ScheduleRule
	holidays map[string]model.Holiday
	shifts   map[uint]model.Shift
	cells    map[string]scheduleCell
}

func newConflictContext(rule model.ScheduleRule, holidays []model.Holiday, shifts []model.Shift, schedules []model.Schedule) *conflictContext {
	c := &conflictContext{
		rule:     rule,
		holidays: map[string]model.Holiday{},
		shifts:   map[uint]model.Shift{},
		cells:    map[string]scheduleCell{},
	}
	if c.rule.MaxConsecutiveDays <= 0 {
		c.rule.MaxConsecutiveDays = 5
	}
	for _, h := range holidays {
		if !h.IsWorkday {
			c.holidays[normDate(h.Date).Format("2006-01-02")] = h
		}
	}
	for _, sh := range shifts {
		c.shifts[sh.ID] = sh
	}
	for _, sc := range schedules {
		c.cells[cellKey(sc.WorkDate, sc.StaffID)] = scheduleCell{id: sc.ID, shiftID: sc.ShiftID, kind: sc.Shift.Kind}
	}
	return c
}

func (c *conflictContext) add(date time.Time, staffID uint, cell scheduleCell) {
	c.cells[cellKey(date, staffID)] = cell
}

// staffConflicts 复核某位员工在当前网格内的所有排班单元，返回 key -> 冲突原因。
func (c *conflictContext) staffConflicts(staffID uint) map[string]ConflictReasons {
	out := map[string]ConflictReasons{}
	for key, cell := range c.cells {
		date, sid := parseCellKey(key)
		if sid != staffID {
			continue
		}
		if reasons := c.reasons(date, sid, cell); len(reasons) > 0 {
			out[key] = reasons
		}
	}
	return out
}

func joinReasons(maps ...map[string]ConflictReasons) ConflictReasons {
	keys := map[string]bool{}
	seen := map[string]bool{}
	var out ConflictReasons
	for _, m := range maps {
		for key, reasons := range m {
			if keys[key] {
				continue
			}
			keys[key] = true
			for _, msg := range reasons {
				if seen[msg] {
					continue
				}
				seen[msg] = true
				out = append(out, msg)
			}
		}
	}
	return out
}

// swapWindow 返回交换复核涉及的统一日期窗口（前后各 contextWindowDays 天）。
func swapWindow(d1, d2 time.Time) (time.Time, time.Time) {
	from := normDate(d1)
	if normDate(d2).Before(from) {
		from = normDate(d2)
	}
	to := normDate(d1)
	if normDate(d2).After(to) {
		to = normDate(d2)
	}
	return from.AddDate(0, 0, -contextWindowDays), to.AddDate(0, 0, contextWindowDays)
}

func (c *conflictContext) kindOf(shiftID uint) model.ShiftKind {
	if sh, ok := c.shifts[shiftID]; ok {
		return sh.Kind
	}
	return ""
}

// reasons 返回将 cell 放在 date/staffID 上时命中的冲突原因（视为已覆盖该格旧值）。
// 判定只依赖已加载的网格窗口；窗口外没有排班即不参与连续计算。
func (c *conflictContext) reasons(date time.Time, staffID uint, cell scheduleCell) ConflictReasons {
	date = normDate(date)
	var out ConflictReasons

	if cell.kind != model.ShiftRest {
		streak := 1
		first := date
		for d := date.AddDate(0, 0, -1); ; d = d.AddDate(0, 0, -1) {
			prev, ok := c.cells[cellKey(d, staffID)]
			if !ok || prev.kind == model.ShiftRest {
				break
			}
			streak++
			first = normDate(d)
		}
		var last time.Time
		for d := date.AddDate(0, 0, 1); ; d = d.AddDate(0, 0, 1) {
			next, ok := c.cells[cellKey(d, staffID)]
			if !ok || next.kind == model.ShiftRest {
				break
			}
			streak++
			last = normDate(d)
		}
		if streak > c.rule.MaxConsecutiveDays {
			end := date
			if !last.IsZero() {
				end = last
			}
			out = append(out, fmt.Sprintf("%s至%s连续工作%d天，超过上限%d天", first.Format("01-02"), end.Format("01-02"), streak, c.rule.MaxConsecutiveDays))
		}
	}

	if c.rule.ForbidNightToDay {
		if cell.kind == model.ShiftDay {
			if prev, ok := c.cells[cellKey(date.AddDate(0, 0, -1), staffID)]; ok && prev.kind == model.ShiftNight {
				out = append(out, fmt.Sprintf("%s夜班后，%s不得排白班", date.AddDate(0, 0, -1).Format("01-02"), date.Format("01-02")))
			}
		}
		if cell.kind == model.ShiftNight {
			if next, ok := c.cells[cellKey(date.AddDate(0, 0, 1), staffID)]; ok && next.kind == model.ShiftDay {
				out = append(out, fmt.Sprintf("%s夜班后，%s不得排白班", date.Format("01-02"), date.AddDate(0, 0, 1).Format("01-02")))
			}
		}
	}

	if c.rule.HolidayPriority && cell.kind != model.ShiftRest {
		if h, ok := c.holidays[date.Format("2006-01-02")]; ok {
			prev, hasPrev := c.cells[cellKey(date.AddDate(0, 0, -1), staffID)]
			if hasPrev && prev.kind != model.ShiftRest {
				if ph, pok := c.holidays[normDate(date.AddDate(0, 0, -1)).Format("2006-01-02")]; pok {
					out = append(out, fmt.Sprintf("法定节假日「%s」与「%s」连续值班，节假日应优先安排休息", ph.Name, h.Name))
				}
			}
		}
	}

	return out
}
