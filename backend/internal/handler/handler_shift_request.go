package handler

import (
	"github.com/gbsched/hospital-scheduler/internal/constants"
	"github.com/gbsched/hospital-scheduler/internal/dto"
	"github.com/gbsched/hospital-scheduler/internal/middleware"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/service"
	"github.com/gbsched/hospital-scheduler/pkg/response"
	"github.com/gin-gonic/gin"
)

type ShiftRequestHandler struct{ s *service.ShiftRequestService }

func NewShiftRequestHandler(s *service.ShiftRequestService) *ShiftRequestHandler {
	return &ShiftRequestHandler{s}
}
func (h *ShiftRequestHandler) List(c *gin.Context) {
	v, e := h.s.List()
	if e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, v)
}
func (h *ShiftRequestHandler) Create(c *gin.Context) {
	var q dto.ShiftRequestCreate
	if e := c.ShouldBindJSON(&q); e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "调班申请参数错误")
		return
	}
	if e := h.s.Create(model.ShiftRequest{ScheduleID: q.ScheduleID, SubstituteID: q.SubstituteID, SubstituteScheduleID: q.SubstituteScheduleID, Reason: q.Reason}, middleware.StaffID(c)); e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, "申请已提交")
}
func (h *ShiftRequestHandler) Review(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var q dto.ReviewRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "审批参数错误")
		return
	}
	if e := h.s.Review(id, middleware.StaffID(c), q.Approved); e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, "审批完成")
}
