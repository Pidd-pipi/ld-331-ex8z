package repository

import (
	"errors"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"gorm.io/gorm"
)

type DepartmentRepository struct{ db *gorm.DB }

func NewDepartmentRepository(db *gorm.DB) *DepartmentRepository { return &DepartmentRepository{db} }
func (r *DepartmentRepository) List() ([]model.Department, error) {
	var v []model.Department
	e := r.db.Preload("Positions").Order("id").Find(&v).Error
	return v, e
}
func (r *DepartmentRepository) Create(v *model.Department) error { return r.db.Create(v).Error }
func (r *DepartmentRepository) Find(id uint) (model.Department, error) {
	var v model.Department
	e := r.db.Preload("Positions").First(&v, id).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return v, ErrNotFound
	}
	return v, e
}
