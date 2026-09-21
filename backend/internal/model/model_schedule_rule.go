package model

type ScheduleRule struct {
	BaseModel
	DepartmentID       uint `gorm:"uniqueIndex" json:"department_id"`
	MaxConsecutiveDays int  `json:"max_consecutive_days"`
	WeekendRotation    bool `json:"weekend_rotation"`
	HolidayPriority    bool `json:"holiday_priority"`
	ForbidNightToDay   bool `json:"forbid_night_to_day"`
}
