package repository

import (
	"errors"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"gorm.io/gorm"
)

type PositionRepository struct{ db *gorm.DB }

func NewPositionRepository(db *gorm.DB) *PositionRepository  { return &PositionRepository{db} }
func (r *PositionRepository) Create(v *model.Position) error { return r.db.Create(v).Error }
func (r *PositionRepository) List(departmentID uint) ([]model.Position, error) {
	var v []model.Position
	q := r.db.Order("id")
	if departmentID > 0 {
		q = q.Where("department_id=?", departmentID)
	}
	return v, q.Find(&v).Error
}
func (r *PositionRepository) Find(id uint) (model.Position, error) {
	var v model.Position
	e := r.db.First(&v, id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, ErrNotFound
	}
	return v, e
}
