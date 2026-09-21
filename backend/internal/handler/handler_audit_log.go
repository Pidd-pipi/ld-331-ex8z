package handler

import (
	"github.com/gbsched/hospital-scheduler/internal/service"
	"github.com/gbsched/hospital-scheduler/pkg/response"
	"github.com/gin-gonic/gin"
)

type AuditHandler struct{ s *service.AuditService }

func NewAuditHandler(s *service.AuditService) *AuditHandler { return &AuditHandler{s} }
func (h *AuditHandler) List(c *gin.Context) {
	v, e := h.s.List()
	if e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, v)
}
