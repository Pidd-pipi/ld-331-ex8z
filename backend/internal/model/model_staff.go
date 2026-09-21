package model

type Role string

const (
	RoleAdmin      Role = "admin"
	RoleSupervisor Role = "supervisor"
	RoleStaff      Role = "staff"
)

type Staff struct {
	BaseModel
	Name         string `json:"name"`
	Username     string `gorm:"uniqueIndex;size:80" json:"username"`
	PasswordHash string `json:"-"`
	Role         Role   `json:"role"`
	DepartmentID uint   `json:"department_id"`
	PositionID   uint   `json:"position_id"`
	Skills       string `json:"skills"`
	Active       bool   `json:"active"`
}
