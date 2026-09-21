package handler

import (
	"github.com/gbsched/hospital-scheduler/internal/constants"
	"github.com/gbsched/hospital-scheduler/internal/dto"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/service"
	"github.com/gbsched/hospital-scheduler/pkg/response"
	"github.com/gin-gonic/gin"
	"strconv"
)

type PositionHandler struct{ s *service.PositionService }

func NewPositionHandler(s *service.PositionService) *PositionHandler { return &PositionHandler{s} }
func (h *PositionHandler) List(c *gin.Context) {
	d, _ := strconv.ParseUint(c.Query("department_id"), 10, 64)
	v, e := h.s.List(uint(d))
	if e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, v)
}
func (h *PositionHandler) Create(c *gin.Context) {
	var q dto.PositionRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "岗位参数错误")
		return
	}
	if e := h.s.Create(model.Position{DepartmentID: q.DepartmentID, Name: q.Name, StaffingQuota: q.StaffingQuota, Skills: q.Skills}); e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, "创建成功")
}
