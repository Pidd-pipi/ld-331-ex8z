package repository

import (
	"errors"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"gorm.io/gorm"
)

type ShiftRepository struct{ db *gorm.DB }

func NewShiftRepository(db *gorm.DB) *ShiftRepository { return &ShiftRepository{db} }
func (r *ShiftRepository) List() ([]model.Shift, error) {
	var v []model.Shift
	return v, r.db.Order("id").Find(&v).Error
}
func (r *ShiftRepository) ByKind(k model.ShiftKind) (model.Shift, error) {
	var v model.Shift
	e := r.db.Where("kind=?", k).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, ErrNotFound
	}
	return v, e
}
