package model

import (
	"time"
)

type Holiday struct {
	BaseModel
	Date       time.Time `gorm:"uniqueIndex" json:"date"`
	Name       string    `json:"name"`
	IsWorkday  bool      `json:"is_workday"`
	Multiplier float64   `json:"multiplier"`
}
