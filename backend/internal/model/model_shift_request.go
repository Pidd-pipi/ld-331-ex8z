package model

import (
	"time"
)

type RequestStatus string

const (
	RequestPending  RequestStatus = "pending"
	RequestApproved RequestStatus = "approved"
	RequestRejected RequestStatus = "rejected"
)

type ShiftRequest struct {
	BaseModel
	ApplicantID          uint          `json:"applicant_id"`
	SubstituteID         uint          `json:"substitute_id"`
	ScheduleID           uint          `json:"schedule_id"`
	SubstituteScheduleID uint          `json:"substitute_schedule_id"`
	Reason               string        `json:"reason"`
	Status               RequestStatus `json:"status"`
	ReviewerID           *uint         `json:"reviewer_id"`
	ReviewedAt           *time.Time    `json:"reviewed_at"`
	Applicant            Staff         `gorm:"foreignKey:ApplicantID" json:"applicant,omitempty"`
	Substitute           Staff         `gorm:"foreignKey:SubstituteID" json:"substitute,omitempty"`
}
