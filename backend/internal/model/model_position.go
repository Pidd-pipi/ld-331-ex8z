package model

type Position struct {
	BaseModel
	DepartmentID  uint   `json:"department_id"`
	Name          string `json:"name"`
	StaffingQuota int    `json:"staffing_quota"`
	Skills        string `json:"skills"`
}
