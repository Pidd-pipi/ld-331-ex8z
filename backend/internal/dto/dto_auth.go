package dto

type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=80"`
	Password string `json:"password" binding:"required,min=6"`
}
type LoginResponse struct {
	Token   string `json:"token"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	StaffID uint   `json:"staff_id"`
}
