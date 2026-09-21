package service

import (
	"fmt"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"log/slog"
)

type HolidayService struct {
	repo   *repository.HolidayRepository
	logger *slog.Logger
}

func NewHolidayService(r *repository.HolidayRepository, l *slog.Logger) *HolidayService {
	return &HolidayService{r, l}
}
func (s *HolidayService) List() ([]model.Holiday, error) {
	v, e := s.repo.List()
	if e != nil {
		return nil, fmt.Errorf("list holidays: %w", e)
	}
	return v, nil
}
func (s *HolidayService) Create(v model.Holiday) error {
	if e := s.repo.Create(&v); e != nil {
		return fmt.Errorf("create holiday: %w", e)
	}
	return nil
}
