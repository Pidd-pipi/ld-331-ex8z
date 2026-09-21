package dto

type ShiftRequestCreate struct {
	ScheduleID           uint   `json:"schedule_id" binding:"required"`
	SubstituteID         uint   `json:"substitute_id" binding:"required"`
	SubstituteScheduleID uint   `json:"substitute_schedule_id" binding:"required"`
	Reason               string `json:"reason" binding:"required,min=2,max=300"`
}
type ReviewRequest struct {
	Approved bool   `json:"approved"`
	Comment  string `json:"comment"`
}
