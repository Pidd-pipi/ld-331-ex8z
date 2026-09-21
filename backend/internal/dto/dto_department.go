package dto

type DepartmentRequest struct {
	Name     string `json:"name" binding:"required,max=100"`
	ParentID *uint  `json:"parent_id"`
}
type PositionRequest struct {
	DepartmentID  uint   `json:"department_id" binding:"required"`
	Name          string `json:"name" binding:"required,max=80"`
	StaffingQuota int    `json:"staffing_quota" binding:"gte=0"`
	Skills        string `json:"skills"`
}
