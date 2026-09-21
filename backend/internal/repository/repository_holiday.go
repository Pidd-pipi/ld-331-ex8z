package repository

import (
	"github.com/gbsched/hospital-scheduler/internal/model"
	"gorm.io/gorm"
)

type HolidayRepository struct{ db *gorm.DB }

func NewHolidayRepository(db *gorm.DB) *HolidayRepository  { return &HolidayRepository{db} }
func (r *HolidayRepository) Create(v *model.Holiday) error { return r.db.Create(v).Error }
func (r *HolidayRepository) List() ([]model.Holiday, error) {
	var v []model.Holiday
	return v, r.db.Order("date").Find(&v).Error
}
