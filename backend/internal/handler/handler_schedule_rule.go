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

type RuleHandler struct{ s *service.RuleService }

func NewRuleHandler(s *service.RuleService) *RuleHandler { return &RuleHandler{s} }
func (h *RuleHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Query("department_id"), 10, 64)
	v, e := h.s.Get(uint(id))
	if e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, v)
}
func (h *RuleHandler) Save(c *gin.Context) {
	var q dto.RuleRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "规则参数错误")
		return
	}
	if e := h.s.Save(model.ScheduleRule{DepartmentID: q.DepartmentID, MaxConsecutiveDays: q.MaxConsecutiveDays, WeekendRotation: q.WeekendRotation, HolidayPriority: q.HolidayPriority, ForbidNightToDay: q.ForbidNightToDay}); e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, "规则已保存")
}
