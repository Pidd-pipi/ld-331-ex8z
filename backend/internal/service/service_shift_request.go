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
	db           *gorm.DB
	logger       *slog.Logger
}

func NewShiftRequestService(r *repository.ShiftRequestRepository, s *repository.ScheduleRepository, db *gorm.DB, l *slog.Logger) *ShiftRequestService {
	return &ShiftRequestService{r, s, db, l}
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
func (s *ShiftRequestService) Review(id, reviewer uint, approved bool) error {
	r, e := s.repo.Find(id)
	if e != nil {
		return fmt.Errorf("find shift request: %w", e)
	}
	if r.Status != model.RequestPending {
		return fmt.Errorf("request already reviewed")
	}
	now := time.Now()
	r.ReviewerID = &reviewer
	r.ReviewedAt = &now
	if !approved {
		r.Status = model.RequestRejected
		return s.repo.Save(&r)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var a, b model.Schedule
		if e := tx.First(&a, r.ScheduleID).Error; e != nil {
			return e
		}
		if e := tx.First(&b, r.SubstituteScheduleID).Error; e != nil {
			return e
		}
		a.StaffID, b.StaffID = b.StaffID, a.StaffID
		if e := tx.Save(&a).Error; e != nil {
			return e
		}
		if e := tx.Save(&b).Error; e != nil {
			return e
		}
		r.Status = model.RequestApproved
		return tx.Save(&r).Error
	})
}
