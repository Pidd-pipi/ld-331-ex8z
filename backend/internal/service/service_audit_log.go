package service

import (
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"log/slog"
)

type AuditService struct {
	repo   *repository.AuditLogRepository
	logger *slog.Logger
}

func NewAuditService(r *repository.AuditLogRepository, l *slog.Logger) *AuditService {
	return &AuditService{r, l}
}
func (s *AuditService) Log(actor uint, action, detail string) {
	if e := s.repo.Create(&model.AuditLog{ActorID: actor, Action: action, Detail: detail}); e != nil {
		s.logger.Error("write audit log", "error", e)
	}
}
func (s *AuditService) List() ([]model.AuditLog, error) { return s.repo.List() }
