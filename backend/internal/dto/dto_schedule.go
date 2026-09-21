package dto

type GenerateScheduleRequest struct {
	DepartmentID uint   `json:"department_id" binding:"required"`
	StartDate    string `json:"start_date" binding:"required,datetime=2006-01-02"`
	EndDate      string `json:"end_date" binding:"required,datetime=2006-01-02"`
}
type UpdateScheduleRequest struct {
	ShiftID uint   `json:"shift_id" binding:"required"`
	Note    string `json:"note"`
}
type RuleRequest struct {
	DepartmentID       uint `json:"department_id" binding:"required"`
	MaxConsecutiveDays int  `json:"max_consecutive_days" binding:"gte=1,lte=14"`
	WeekendRotation    bool `json:"weekend_rotation"`
	HolidayPriority    bool `json:"holiday_priority"`
	ForbidNightToDay   bool `json:"forbid_night_to_day"`
}
