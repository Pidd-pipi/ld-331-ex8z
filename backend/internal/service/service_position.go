package service

import (
	"fmt"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"log/slog"
)

type PositionService struct {
	repo   *repository.PositionRepository
	logger *slog.Logger
}

func NewPositionService(r *repository.PositionRepository, l *slog.Logger) *PositionService {
	return &PositionService{r, l}
}
func (s *PositionService) List(dept uint) ([]model.Position, error) {
	v, e := s.repo.List(dept)
	if e != nil {
		return nil, fmt.Errorf("list positions: %w", e)
	}
	return v, nil
}
func (s *PositionService) Create(v model.Position) error {
	if e := s.repo.Create(&v); e != nil {
		return fmt.Errorf("create position: %w", e)
	}
	return nil
}
