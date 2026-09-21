package service

import (
	"fmt"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"log/slog"
)

type DepartmentService struct {
	repo   *repository.DepartmentRepository
	logger *slog.Logger
}

func NewDepartmentService(r *repository.DepartmentRepository, l *slog.Logger) *DepartmentService {
	return &DepartmentService{r, l}
}
func (s *DepartmentService) List() ([]model.Department, error) {
	v, e := s.repo.List()
	if e != nil {
		return nil, fmt.Errorf("list departments: %w", e)
	}
	return v, nil
}
func (s *DepartmentService) Create(v model.Department) error {
	if e := s.repo.Create(&v); e != nil {
		return fmt.Errorf("create department: %w", e)
	}
	return nil
}
