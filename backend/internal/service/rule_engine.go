package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
)

// Conflict codes are embedded into the human-readable reason so the generated
// schedule rows stay compatible with the existing {code,message,data} contract.
const (
	reasonConsecutive = "连续工作超过%d天上限"
	reasonNightToDay  = "夜班后次日不得安排白班"
	reasonHoliday     = "节假日应优先安排休息（节假日休息次数少于在岗休息人员）"
)

// ScheduleConflict carries one rule violation. It is keyed by schedule id so
// callers can persist the reason back onto the schedule row.
type ScheduleConflict struct {
	ScheduleID uint
	StaffID    uint
	WorkDate   time.Time
	Reason     string
}

func (c ScheduleConflict) Error() string {
	return fmt.Sprintf("%s 排班冲突：%s", c.WorkDate.Format("2006-01-02"), c.Reason)
}

// ScheduleConflictError is returned when a manual adjustment or a shift-swap
// approval would violate the latest rules. Callers must reject the whole
// operation without mutating existing schedules or request status.
type ScheduleConflictError struct {
	Conflicts []ScheduleConflict
}

func (e *ScheduleConflictError) Error() string {
	if e == nil || len(e.Conflicts) == 0 {
		return "排班冲突"
	}
	parts := make([]string, 0, len(e.Conflicts))
	shown := map[string]bool{}
	for _, c := range e.Conflicts {
		line := fmt.Sprintf("%s %s", c.WorkDate.Format("2006-01-02"), c.Reason)
		if !shown[line] {
			parts = append(parts, line)
			shown[line] = true
		}
		if len(parts) >= 3 {
			break
		}
	}
	return "排班冲突，已整次拒绝：" + strings.Join(parts, "；")
}

// RuleEngine evaluates the currently configured scheduling rules for one
// department. It is the single source of truth shared by auto generation,
// manual adjustment and shift-swap approval.
type RuleEngine struct {
	schedRepo   *repository.ScheduleRepository
	ruleRepo    *repository.ScheduleRuleRepository
	holidayRepo *repository.HolidayRepository
}

func NewRuleEngine(sr *repository.ScheduleRepository, rr *repository.ScheduleRuleRepository, hr *repository.HolidayRepository) *RuleEngine {
	return &RuleEngine{schedRepo: sr, ruleRepo: rr, holidayRepo: hr}
}

type ruleConfig struct {
	maxConsecutive  int
	nightToDay      bool
	holidayPriority bool
}

func (e *RuleEngine) config(dept uint) ruleConfig {
	cfg := ruleConfig{maxConsecutive: 5}
	if rule, err := e.ruleRepo.ByDepartment(dept); err == nil {
		if rule.MaxConsecutiveDays > 0 {
			cfg.maxConsecutive = rule.MaxConsecutiveDays
		}
		cfg.nightToDay = rule.ForbidNightToDay
		cfg.holidayPriority = rule.HolidayPriority
	}
	return cfg
}

type schedKey struct {
	date  time.Time
	staff uint
}

// Check evaluates rules for department dept over [from,to]. overrides replace
// or supplement the persisted rows (used to simulate an edit/swap before the
// transaction is committed). The returned map is keyed by schedule id.
func (e *RuleEngine) Check(dept uint, from, to time.Time, overrides []model.Schedule) (map[uint]string, error) {
	cfg := e.config(dept)
	padDays := cfg.maxConsecutive + 1
	qFrom, qTo := from.AddDate(0, 0, -padDays), to.AddDate(0, 0, padDays)

	existing, err := e.schedRepo.List(dept, 0, truncateDay(qFrom), truncateDay(qTo))
	if err != nil {
		return nil, fmt.Errorf("load schedules for rule check: %w", err)
	}
	holidays, err := e.holidayRepo.List()
	if err != nil {
		return nil, fmt.Errorf("load holidays for rule check: %w", err)
	}
	holidaySet := map[string]bool{}
	for _, h := range holidays {
		if !h.IsWorkday && !h.Date.Before(qFrom) && !h.Date.After(qTo) {
			holidaySet[dayKey(h.Date)] = true
		}
	}

	// Rows are merged by schedule id first so edits/swaps replace the
	// original row instead of coexisting with its stale key.
	byID := map[uint]model.Schedule{}
	var synthetic []model.Schedule
	for _, s := range existing {
		byID[s.ID] = s
	}
	for _, s := range overrides {
		if s.ID == 0 {
			synthetic = append(synthetic, s)
		} else {
			byID[s.ID] = s
		}
	}
	entries := map[schedKey]model.Schedule{}
	for _, s := range byID {
		entries[schedKey{truncateDay(s.WorkDate), s.StaffID}] = s
	}
	for _, s := range synthetic {
		entries[schedKey{truncateDay(s.WorkDate), s.StaffID}] = s
	}

	byStaff := map[uint][]model.Schedule{}
	for _, s := range entries {
		byStaff[s.StaffID] = append(byStaff[s.StaffID], s)
	}
	for staff := range byStaff {
		sort.Slice(byStaff[staff], func(i, j int) bool { return byStaff[staff][i].WorkDate.Before(byStaff[staff][j].WorkDate) })
	}

	reasons := map[uint]string{}
	addReason := func(s model.Schedule, reason string) {
		if s.ID == 0 {
			return
		}
		if cur := reasons[s.ID]; cur != "" {
			reasons[s.ID] = cur + "；" + reason
		} else {
			reasons[s.ID] = reason
		}
	}

	inWindow := func(t time.Time) bool {
		t = truncateDay(t)
		return !t.Before(truncateDay(from)) && !t.After(truncateDay(to))
	}

	// Rule 1: maximum consecutive working days. A rest day resets the streak;
	// a missing row (no schedule generated for that day) also resets it.
	for staff := range byStaff {
		streak := 0
		for d := qFrom; !d.After(qTo); d = d.AddDate(0, 0, 1) {
			s, ok := entries[schedKey{truncateDay(d), staff}]
			working := ok && s.Shift.Kind != model.ShiftRest
			if !working {
				streak = 0
				continue
			}
			streak++
			if streak > cfg.maxConsecutive && inWindow(s.WorkDate) {
				addReason(s, fmt.Sprintf(reasonConsecutive, cfg.maxConsecutive))
			}
		}
	}

	// Rule 2: a night shift must not be followed by a day shift.
	if cfg.nightToDay {
		for _, list := range byStaff {
			for i := 1; i < len(list); i++ {
				prev, cur := list[i-1], list[i]
				if truncateDay(cur.WorkDate).Sub(truncateDay(prev.WorkDate)) != 24*time.Hour {
					continue
				}
				if prev.Shift.Kind == model.ShiftNight && cur.Shift.Kind == model.ShiftDay {
					if inWindow(cur.WorkDate) {
						addReason(cur, reasonNightToDay)
					}
				}
			}
		}
	}

	// Rule 3: holiday priority — staff with fewer accumulated holiday rests
	// take priority to rest on a public holiday. A working member conflicts
	// when someone resting that same holiday has had at least as many holiday
	// rests as they have.
	if cfg.holidayPriority {
		staffSet := map[uint]bool{}
		for staff := range byStaff {
			staffSet[staff] = true
		}
		for d := qFrom; !d.After(qTo); d = d.AddDate(0, 0, 1) {
			if !holidaySet[dayKey(d)] {
				continue
			}
			beforeRests := map[uint]int{}
			var workers, resters []model.Schedule
			for staff := range staffSet {
				cnt := 0
				for hd := qFrom; hd.Before(truncateDay(d)); hd = hd.AddDate(0, 0, 1) {
					if !holidaySet[dayKey(hd)] {
						continue
					}
					if s, ok := entries[schedKey{truncateDay(hd), staff}]; ok && s.Shift.Kind == model.ShiftRest {
						cnt++
					}
				}
				beforeRests[staff] = cnt
				if s, ok := entries[schedKey{truncateDay(d), staff}]; ok {
					if s.Shift.Kind == model.ShiftRest {
						resters = append(resters, s)
					} else {
						workers = append(workers, s)
					}
				}
			}
			maxRester := -1
			for _, r := range resters {
				if beforeRests[r.StaffID] > maxRester {
					maxRester = beforeRests[r.StaffID]
				}
			}
			if maxRester < 0 {
				continue
			}
			for _, w := range workers {
				if inWindow(w.WorkDate) && beforeRests[w.StaffID] < maxRester {
					addReason(w, reasonHoliday)
				}
			}
		}
	}

	return reasons, nil
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func dayKey(t time.Time) string {
	return truncateDay(t).Format("2006-01-02")
}
