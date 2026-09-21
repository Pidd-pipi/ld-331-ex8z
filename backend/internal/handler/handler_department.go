package handler

import (
	"github.com/gbsched/hospital-scheduler/internal/constants"
	"github.com/gbsched/hospital-scheduler/internal/dto"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/service"
	"github.com/gbsched/hospital-scheduler/pkg/response"
	"github.com/gin-gonic/gin"
)

type DepartmentHandler struct{ s *service.DepartmentService }

func NewDepartmentHandler(s *service.DepartmentService) *DepartmentHandler {
	return &DepartmentHandler{s}
}
func (h *DepartmentHandler) List(c *gin.Context) {
	v, e := h.s.List()
	if e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, v)
}
func (h *DepartmentHandler) Create(c *gin.Context) {
	var q dto.DepartmentRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "科室参数错误")
		return
	}
	if e := h.s.Create(model.Department{Name: q.Name, ParentID: q.ParentID}); e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, "创建成功")
}
