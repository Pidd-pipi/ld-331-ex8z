package repository

import (
	"errors"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"gorm.io/gorm"
)

type StaffRepository struct{ db *gorm.DB }

func NewStaffRepository(db *gorm.DB) *StaffRepository  { return &StaffRepository{db} }
func (r *StaffRepository) Create(v *model.Staff) error { return r.db.Create(v).Error }
func (r *StaffRepository) Find(id uint) (model.Staff, error) {
	var v model.Staff
	e := r.db.First(&v, id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, ErrNotFound
	}
	return v, e
}
func (r *StaffRepository) ByUsername(s string) (model.Staff, error) {
	var v model.Staff
	e := r.db.Where("username=?", s).First(&v).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, ErrNotFound
	}
	return v, e
}
func (r *StaffRepository) List(dept uint) ([]model.Staff, error) {
	var v []model.Staff
	q := r.db.Where("active=?", true).Order("id")
	if dept > 0 {
		q = q.Where("department_id=?", dept)
	}
	return v, q.Find(&v).Error
}
