package service

import (
	"fmt"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"log/slog"
	"sort"
	"time"
)

type ScheduleService struct {
	repo      *repository.ScheduleRepository
	staffRepo *repository.StaffRepository
	shiftRepo *repository.ShiftRepository
	ruleRepo  *repository.ScheduleRuleRepository
	logger    *slog.Logger
}

func NewScheduleService(r *repository.ScheduleRepository, sr *repository.StaffRepository, sh *repository.ShiftRepository, rr *repository.ScheduleRuleRepository, l *slog.Logger) *ScheduleService {
	return &ScheduleService{r, sr, sh, rr, l}
}
func (s *ScheduleService) List(dept, staff uint, from, to time.Time) ([]model.Schedule, error) {
	v, e := s.repo.List(dept, staff, from, to)
	if e != nil {
		return nil, fmt.Errorf("list schedules: %w", e)
	}
	return v, nil
}
func (s *ScheduleService) Generate(dept uint, from, to time.Time) (int, error) {
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
	rule, _ := s.ruleRepo.ByDepartment(dept)
	if rule.MaxConsecutiveDays == 0 {
		rule.MaxConsecutiveDays = 5
	}
	if e = s.repo.DeleteRange(dept, from, to); e != nil {
		return 0, fmt.Errorf("clear range: %w", e)
	}
	created := 0
	consecutive := map[uint]int{}
	for d, i := from, 0; !d.After(to); d, i = d.AddDate(0, 0, 1), i+1 {
		for j, staff := range staffs {
			shift := day
			if consecutive[staff.ID] >= rule.MaxConsecutiveDays {
				shift = rest
				consecutive[staff.ID] = 0
			} else if (i+j)%5 == 0 {
				shift = night
				consecutive[staff.ID]++
			} else {
				consecutive[staff.ID]++
			}
			if e = s.repo.Create(&model.Schedule{DepartmentID: dept, StaffID: staff.ID, ShiftID: shift.ID, WorkDate: d}); e != nil {
				return created, fmt.Errorf("create schedule: %w", e)
			}
			created++
		}
	}
	return created, nil
}
func (s *ScheduleService) Update(id, shiftID uint, note string) error {
	v, e := s.repo.Find(id)
	if e != nil {
		return fmt.Errorf("find schedule: %w", e)
	}
	v.ShiftID = shiftID
	v.Note = note
	if e = s.repo.Save(&v); e != nil {
		return fmt.Errorf("update schedule: %w", e)
	}
	return nil
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
