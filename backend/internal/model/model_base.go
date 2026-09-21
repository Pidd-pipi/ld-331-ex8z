package model

import "time"

// BaseModel keeps API identifiers lower-cased while retaining standard GORM timestamps.
type BaseModel struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"-"`
}
