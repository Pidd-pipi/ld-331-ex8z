package model

type AuditLog struct {
	BaseModel
	ActorID uint   `json:"actor_id"`
	Action  string `json:"action"`
	Detail  string `json:"detail"`
}
