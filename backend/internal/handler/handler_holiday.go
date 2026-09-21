package handler

import (
	"github.com/gbsched/hospital-scheduler/internal/constants"
	"github.com/gbsched/hospital-scheduler/internal/dto"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/service"
	"github.com/gbsched/hospital-scheduler/pkg/response"
	"github.com/gin-gonic/gin"
	"time"
)

type HolidayHandler struct{ s *service.HolidayService }

func NewHolidayHandler(s *service.HolidayService) *HolidayHandler { return &HolidayHandler{s} }
func (h *HolidayHandler) List(c *gin.Context) {
	v, e := h.s.List()
	if e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, v)
}
func (h *HolidayHandler) Create(c *gin.Context) {
	var q dto.HolidayRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "特殊日期参数错误")
		return
	}
	d, _ := time.Parse("2006-01-02", q.Date)
	if e := h.s.Create(model.Holiday{Date: d, Name: q.Name, IsWorkday: q.IsWorkday, Multiplier: q.Multiplier}); e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, "特殊日期已创建")
}
