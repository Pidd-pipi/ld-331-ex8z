package service

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"gorm.io/gorm"
)

type ShiftRequestService struct {
	repo      *repository.ShiftRequestRepository
	schedRepo *repository.ScheduleRepository
	shiftRepo *repository.ShiftRepository
	engine    *RuleEngine
	db        *gorm.DB
	logger    *slog.Logger
}

func NewShiftRequestService(r *repository.ShiftRequestRepository, s *repository.ScheduleRepository, sh *repository.ShiftRepository, eng *RuleEngine, db *gorm.DB, l *slog.Logger) *ShiftRequestService {
	return &ShiftRequestService{r, s, sh, eng, db, l}
}
func (s *ShiftRequestService) List() ([]model.ShiftRequest, error) {
	v, e := s.repo.List()
	if e != nil {
		return nil, fmt.Errorf("list shift requests: %w", e)
	}
	return v, nil
}
func (s *ShiftRequestService) Create(v model.ShiftRequest, actor uint) error {
	a, e := s.schedRepo.Find(v.ScheduleID)
	if e != nil {
		return fmt.Errorf("find applicant schedule: %w", e)
	}
	b, e := s.schedRepo.Find(v.SubstituteScheduleID)
	if e != nil {
		return fmt.Errorf("find substitute schedule: %w", e)
	}
	if a.StaffID != actor || b.StaffID != v.SubstituteID {
		return fmt.Errorf("schedule ownership does not match applicants")
	}
	v.ApplicantID = actor
	v.Status = model.RequestPending
	if e = s.repo.Create(&v); e != nil {
		return fmt.Errorf("create shift request: %w", e)
	}
	return nil
}

// Review approves or rejects a swap. An approval is simulated against the
// latest scheduling rules before any write: when the swap would create a
// conflict the whole operation is rejected, leaving both schedules and the
// request status untouched.
func (s *ShiftRequestService) Review(id, reviewer uint, approved bool) error {
	r, e := s.repo.Find(id)
	if e != nil {
		return fmt.Errorf("find shift request: %w", e)
	}
	if r.Status != model.RequestPending {
		return fmt.Errorf("request already reviewed")
	}
	if !approved {
		now := time.Now()
		r.ReviewerID = &reviewer
		r.ReviewedAt = &now
		r.Status = model.RequestRejected
		if e := s.repo.Save(&r); e != nil {
			return fmt.Errorf("save rejection: %w", e)
		}
		return nil
	}

	a, err := s.schedRepo.Find(r.ScheduleID)
	if err != nil {
		return fmt.Errorf("find applicant schedule: %w", err)
	}
	b, err := s.schedRepo.Find(r.SubstituteScheduleID)
	if err != nil {
		return fmt.Errorf("find substitute schedule: %w", err)
	}
	if a.DepartmentID != b.DepartmentID {
		return fmt.Errorf("跨科室班次不能直接调换")
	}
	// Find does not preload shifts; the rule engine needs them to evaluate.
	if a.Shift, err = s.shiftRepo.ByID(a.ShiftID); err != nil {
		return fmt.Errorf("load applicant shift: %w", err)
	}
	if b.Shift, err = s.shiftRepo.ByID(b.ShiftID); err != nil {
		return fmt.Errorf("load substitute shift: %w", err)
	}

	// Simulate the swap: rows keep their date/shift, only staff changes.
	swappedA, swappedB := a, b
	swappedA.StaffID, swappedB.StaffID = b.StaffID, a.StaffID

	lo, hi := minTime(a.WorkDate, b.WorkDate), maxTime(a.WorkDate, b.WorkDate)
	pad := s.engine.config(a.DepartmentID).maxConsecutive + 1
	windowFrom := truncateDay(lo).AddDate(0, 0, -(pad + 1))
	windowTo := truncateDay(hi).AddDate(0, 0, pad+1)

	baseline, err := s.engine.Check(a.DepartmentID, windowFrom, windowTo, nil)
	if err != nil {
		return err
	}
	simulated, err := s.engine.Check(a.DepartmentID, windowFrom, windowTo, []model.Schedule{swappedA, swappedB})
	if err != nil {
		return err
	}
	if conflicts := newConflicts(a.DepartmentID, baseline, simulated, s.schedRepo, windowFrom, windowTo); len(conflicts) > 0 {
		return &ScheduleConflictError{Conflicts: conflicts}
	}

	now := time.Now()
	return s.db.Transaction(func(tx *gorm.DB) error {
		var ta, tb model.Schedule
		if e := tx.First(&ta, r.ScheduleID).Error; e != nil {
			return e
		}
		if e := tx.First(&tb, r.SubstituteScheduleID).Error; e != nil {
			return e
		}
		ta.StaffID, tb.StaffID = tb.StaffID, ta.StaffID
		if e := tx.Save(&ta).Error; e != nil {
			return e
		}
		if e := tx.Save(&tb).Error; e != nil {
			return e
		}
		r.ReviewerID = &reviewer
		r.ReviewedAt = &now
		r.Status = model.RequestApproved
		if e := tx.Save(&r).Error; e != nil {
			return e
		}
		// Persist the simulated conflict reasons (state now matches reality).
		txRepo := repository.NewScheduleRepository(tx)
		rows, err := txRepo.List(a.DepartmentID, 0, windowFrom, windowTo)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if e := tx.Model(&model.Schedule{}).Where("id=?", row.ID).Update("conflict_reason", simulated[row.ID]).Error; e != nil {
				return e
			}
		}
		return nil
	})
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
