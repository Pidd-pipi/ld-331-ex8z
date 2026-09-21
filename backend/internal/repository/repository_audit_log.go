package repository

import (
	"github.com/gbsched/hospital-scheduler/internal/model"
	"gorm.io/gorm"
)

type AuditLogRepository struct{ db *gorm.DB }

func NewAuditLogRepository(db *gorm.DB) *AuditLogRepository  { return &AuditLogRepository{db} }
func (r *AuditLogRepository) Create(v *model.AuditLog) error { return r.db.Create(v).Error }
func (r *AuditLogRepository) List() ([]model.AuditLog, error) {
	var v []model.AuditLog
	return v, r.db.Order("id desc").Limit(100).Find(&v).Error
}
