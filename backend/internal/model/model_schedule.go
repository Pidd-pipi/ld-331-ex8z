package model

import (
	"time"
)

type Schedule struct {
	BaseModel
	DepartmentID uint      `gorm:"index" json:"department_id"`
	StaffID      uint      `gorm:"index" json:"staff_id"`
	ShiftID      uint      `json:"shift_id"`
	WorkDate     time.Time `gorm:"index" json:"work_date"`
	Note         string    `json:"note"`
	Staff        Staff     `json:"staff,omitempty"`
	Shift        Shift     `json:"shift,omitempty"`
}
