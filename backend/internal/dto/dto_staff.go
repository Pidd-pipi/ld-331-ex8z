package dto

type StaffRequest struct {
	Name         string `json:"name" binding:"required,max=80"`
	Username     string `json:"username" binding:"required,min=3,max=80"`
	Password     string `json:"password" binding:"required,min=6"`
	Role         string `json:"role" binding:"required,oneof=admin supervisor staff"`
	DepartmentID uint   `json:"department_id" binding:"required"`
	PositionID   uint   `json:"position_id" binding:"required"`
	Skills       string `json:"skills"`
}
