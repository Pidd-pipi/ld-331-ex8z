package repository

import (
	"errors"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"gorm.io/gorm"
	"time"
)

type ScheduleRepository struct{ db *gorm.DB }

func NewScheduleRepository(db *gorm.DB) *ScheduleRepository  { return &ScheduleRepository{db} }
func (r *ScheduleRepository) Create(v *model.Schedule) error { return r.db.Create(v).Error }
func (r *ScheduleRepository) Save(v *model.Schedule) error   { return r.db.Save(v).Error }
func (r *ScheduleRepository) List(dept, staff uint, from, to time.Time) ([]model.Schedule, error) {
	var v []model.Schedule
	q := r.db.Preload("Staff").Preload("Shift").Order("work_date,staff_id")
	if dept > 0 {
		q = q.Where("department_id=?", dept)
	}
	if staff > 0 {
		q = q.Where("staff_id=?", staff)
	}
	if !from.IsZero() {
		q = q.Where("work_date>=?", from)
	}
	if !to.IsZero() {
		q = q.Where("work_date<=?", to)
	}
	return v, q.Find(&v).Error
}
func (r *ScheduleRepository) Find(id uint) (model.Schedule, error) {
	var v model.Schedule
	e := r.db.First(&v, id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, ErrNotFound
	}
	return v, e
}
func (r *ScheduleRepository) DeleteRange(dept uint, from, to time.Time) error {
	return r.db.Where("department_id=? AND work_date>=? AND work_date<=?", dept, from, to).Delete(&model.Schedule{}).Error
}
