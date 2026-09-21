package repository

import (
	"errors"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"gorm.io/gorm"
)

type ScheduleRuleRepository struct{ db *gorm.DB }

func NewScheduleRuleRepository(db *gorm.DB) *ScheduleRuleRepository {
	return &ScheduleRuleRepository{db}
}
func (r *ScheduleRuleRepository) Upsert(v *model.ScheduleRule) error {
	var existing model.ScheduleRule
	e := r.db.Where("department_id=?", v.DepartmentID).First(&existing).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return r.db.Create(v).Error
	}
	if e != nil {
		return e
	}
	attrs := map[string]any{
		"max_consecutive_days": v.MaxConsecutiveDays,
		"weekend_rotation":     v.WeekendRotation,
		"holiday_priority":     v.HolidayPriority,
		"forbid_night_to_day":  v.ForbidNightToDay,
	}
	return r.db.Model(&existing).Updates(attrs).Error
}
func (r *ScheduleRuleRepository) ByDepartment(id uint) (model.ScheduleRule, error) {
	var v model.ScheduleRule
	e := r.db.Where("department_id=?", id).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, ErrNotFound
	}
	return v, e
}
