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

type StaffHandler struct{ s *service.StaffService }

func NewStaffHandler(s *service.StaffService) *StaffHandler { return &StaffHandler{s} }
func (h *StaffHandler) List(c *gin.Context) {
	d, _ := strconv.ParseUint(c.Query("department_id"), 10, 64)
	v, e := h.s.List(uint(d))
	if e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, v)
}
func (h *StaffHandler) Create(c *gin.Context) {
	var q dto.StaffRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "人员参数错误")
		return
	}
	if e := h.s.Create(model.Staff{Name: q.Name, Username: q.Username, Role: model.Role(q.Role), DepartmentID: q.DepartmentID, PositionID: q.PositionID, Skills: q.Skills}, q.Password); e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, "创建成功")
}
