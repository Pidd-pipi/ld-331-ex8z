package repository

import (
	"errors"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"gorm.io/gorm"
)

type ShiftRequestRepository struct{ db *gorm.DB }

func NewShiftRequestRepository(db *gorm.DB) *ShiftRequestRepository {
	return &ShiftRequestRepository{db}
}
func (r *ShiftRequestRepository) Create(v *model.ShiftRequest) error { return r.db.Create(v).Error }
func (r *ShiftRequestRepository) Save(v *model.ShiftRequest) error   { return r.db.Save(v).Error }
func (r *ShiftRequestRepository) Find(id uint) (model.ShiftRequest, error) {
	var v model.ShiftRequest
	e := r.db.Preload("Applicant").Preload("Substitute").First(&v, id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, ErrNotFound
	}
	return v, e
}
func (r *ShiftRequestRepository) List() ([]model.ShiftRequest, error) {
	var v []model.ShiftRequest
	return v, r.db.Preload("Applicant").Preload("Substitute").Order("id desc").Find(&v).Error
}
