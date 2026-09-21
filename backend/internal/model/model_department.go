package model

type Department struct {
	BaseModel
	Name      string     `gorm:"uniqueIndex;size:100" json:"name"`
	ParentID  *uint      `json:"parent_id"`
	Positions []Position `json:"positions,omitempty"`
}
