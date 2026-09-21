package model

type ShiftKind string

const (
	ShiftDay     ShiftKind = "day"
	ShiftEvening ShiftKind = "evening"
	ShiftNight   ShiftKind = "night"
	ShiftRest    ShiftKind = "rest"
)

type Shift struct {
	BaseModel
	Name      string    `json:"name"`
	Kind      ShiftKind `gorm:"uniqueIndex;size:20" json:"kind"`
	Color     string    `json:"color"`
	StartTime string    `json:"start_time"`
	EndTime   string    `json:"end_time"`
	Hours     float64   `json:"hours"`
}
