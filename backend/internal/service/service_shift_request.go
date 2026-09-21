package service

import (
	"fmt"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"gorm.io/gorm"
	"log/slog"
	"time"
)

type ShiftRequestService struct {
	repo         *repository.ShiftRequestRepository
	scheduleRepo *repository.ScheduleRepository
	scheduleSvc  *ScheduleService
	db           *gorm.DB
	logger       *slog.Logger
}

func NewShiftRequestService(r *repository.ShiftRequestRepository, s *repository.ScheduleRepository, svc *ScheduleService, db *gorm.DB, l *slog.Logger) *ShiftRequestService {
	return &ShiftRequestService{r, s, svc, db, l}
}
func (s *ShiftRequestService) List() ([]model.ShiftRequest, error) {
	v, e := s.repo.List()
	if e != nil {
		return nil, fmt.Errorf("list shift requests: %w", e)
	}
	return v, nil
}
func (s *ShiftRequestService) Create(v model.ShiftRequest, actor uint) error {
	a, e := s.scheduleRepo.Find(v.ScheduleID)
	if e != nil {
		return fmt.Errorf("find applicant schedule: %w", e)
	}
	b, e := s.scheduleRepo.Find(v.SubstituteScheduleID)
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

// Review 审批前按最新规则重查交换后的两张排班：任何冲突都会让整个事务回滚，
// 原班表不动、申请状态保持 pending。审批人与审批时间也只在交换成功后落库。
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
		if e = s.repo.Save(&r); e != nil {
			return fmt.Errorf("save shift request: %w", e)
		}
		return nil
	}
	var conflictErr error
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var a, b model.Schedule
		if e := tx.First(&a, r.ScheduleID).Error; e != nil {
			return fmt.Errorf("load applicant schedule: %w", e)
		}
		if e := tx.First(&b, r.SubstituteScheduleID).Error; e != nil {
			return fmt.Errorf("load substitute schedule: %w", e)
		}
		if err := s.scheduleSvc.ValidateSwap(tx, a, b); err != nil {
			conflictErr = err
			return err
		}
		applicantStaffID := a.StaffID
		substituteStaffID := b.StaffID
		a.StaffID, b.StaffID = substituteStaffID, applicantStaffID
		// 复核已证明交换后无冲突，清除两位员工窗口内遗留的冲突标记。
		if e := tx.Save(&a).Error; e != nil {
			return fmt.Errorf("swap applicant schedule: %w", e)
		}
		if e := tx.Save(&b).Error; e != nil {
			return fmt.Errorf("swap substitute schedule: %w", e)
		}
		winFrom, winTo := swapWindow(a.WorkDate, b.WorkDate)
		if e := s.scheduleRepo.WithTx(tx).ClearConflicts(a.DepartmentID, applicantStaffID, winFrom, winTo); e != nil {
			return fmt.Errorf("clear applicant conflict flags: %w", e)
		}
		if e := s.scheduleRepo.WithTx(tx).ClearConflicts(a.DepartmentID, substituteStaffID, winFrom, winTo); e != nil {
			return fmt.Errorf("clear substitute conflict flags: %w", e)
		}
		now := time.Now()
		r.ReviewerID = &reviewer
		r.ReviewedAt = &now
		r.Status = model.RequestApproved
		if e := tx.Save(&r).Error; e != nil {
			return fmt.Errorf("approve shift request: %w", e)
		}
		return nil
	})
	if conflictErr != nil {
		return conflictErr
	}
	if err != nil {
		return fmt.Errorf("review shift request: %w", err)
	}
	return nil
}
