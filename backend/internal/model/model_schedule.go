package model

import (
	"time"
)

type Schedule struct {
	BaseModel
	DepartmentID    uint      `gorm:"index" json:"department_id"`
	StaffID         uint      `gorm:"index" json:"staff_id"`
	ShiftID         uint      `json:"shift_id"`
	WorkDate        time.Time `gorm:"index" json:"work_date"`
	Note            string    `json:"note"`
	HasConflict     bool      `gorm:"index" json:"has_conflict"`
	ConflictReasons string    `gorm:"size:500" json:"conflict_reasons"`
	Staff           Staff     `json:"staff,omitempty"`
	Shift           Shift     `json:"shift,omitempty"`
}
