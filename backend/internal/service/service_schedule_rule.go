package service

import (
	"fmt"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"log/slog"
)

type RuleService struct {
	repo   *repository.ScheduleRuleRepository
	logger *slog.Logger
}

func NewRuleService(r *repository.ScheduleRuleRepository, l *slog.Logger) *RuleService {
	return &RuleService{r, l}
}
func (s *RuleService) Save(v model.ScheduleRule) error {
	if e := s.repo.Upsert(&v); e != nil {
		return fmt.Errorf("save schedule rule: %w", e)
	}
	return nil
}
func (s *RuleService) Get(dept uint) (model.ScheduleRule, error) {
	v, e := s.repo.ByDepartment(dept)
	if e != nil {
		return v, fmt.Errorf("get schedule rule: %w", e)
	}
	return v, nil
}
