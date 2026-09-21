package handler

import (
	"fmt"
	"github.com/gbsched/hospital-scheduler/internal/constants"
	"github.com/gbsched/hospital-scheduler/internal/dto"
	"github.com/gbsched/hospital-scheduler/internal/service"
	"github.com/gbsched/hospital-scheduler/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"net/http"
	"strconv"
	"time"
)

type ScheduleHandler struct{ s *service.ScheduleService }

func NewScheduleHandler(s *service.ScheduleService) *ScheduleHandler { return &ScheduleHandler{s} }
func (h *ScheduleHandler) List(c *gin.Context) {
	d, _ := strconv.ParseUint(c.Query("department_id"), 10, 64)
	st, _ := strconv.ParseUint(c.Query("staff_id"), 10, 64)
	from, _ := time.Parse("2006-01-02", c.Query("from"))
	to, _ := time.Parse("2006-01-02", c.Query("to"))
	v, e := h.s.List(uint(d), uint(st), from, to)
	if e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, v)
}
func (h *ScheduleHandler) Generate(c *gin.Context) {
	var q dto.GenerateScheduleRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "排班日期或科室不正确")
		return
	}
	f, _ := time.Parse("2006-01-02", q.StartDate)
	t, _ := time.Parse("2006-01-02", q.EndDate)
	n, e := h.s.Generate(q.DepartmentID, f, t)
	if e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, gin.H{"created": n})
}
func (h *ScheduleHandler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var q dto.UpdateScheduleRequest
	if e := c.ShouldBindJSON(&q); e != nil {
		response.Error(c, http.StatusBadRequest, constants.CodeBadRequest, "班次参数错误")
		return
	}
	if e := h.s.Update(id, q.ShiftID, q.Note); e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, "更新成功")
}
func (h *ScheduleHandler) Stats(c *gin.Context) {
	d, _ := strconv.ParseUint(c.Query("department_id"), 10, 64)
	m, e := time.Parse("2006-01", c.DefaultQuery("month", time.Now().Format("2006-01")))
	if e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "月份格式应为 YYYY-MM")
		return
	}
	v, e := h.s.Stats(uint(d), m)
	if e != nil {
		handleError(c, e)
		return
	}
	response.OK(c, v)
}

func (h *ScheduleHandler) Export(c *gin.Context) {
	d, _ := strconv.ParseUint(c.Query("department_id"), 10, 64)
	m, err := time.Parse("2006-01", c.DefaultQuery("month", time.Now().Format("2006-01")))
	if err != nil {
		response.Error(c, http.StatusBadRequest, constants.CodeBadRequest, "月份格式应为 YYYY-MM")
		return
	}
	items, err := h.s.Stats(uint(d), m)
	if err != nil {
		handleError(c, err)
		return
	}
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	headers := []string{"人员", "出勤天数", "白班", "中班", "夜班", "加班时长(小时)"}
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		response.Error(c, 500, constants.CodeInternal, "创建导出文件失败")
		return
	}
	for i, row := range items {
		values := []any{row["name"], row["attendance_days"], row["day_shifts"], row["evening_shifts"], row["night_shifts"], row["overtime_hours"]}
		if err := f.SetSheetRow(sheet, fmt.Sprintf("A%d", i+2), &values); err != nil {
			response.Error(c, 500, constants.CodeInternal, "写入导出数据失败")
			return
		}
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=statistics-%s.xlsx", m.Format("2006-01")))
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err := f.Write(c.Writer); err != nil {
		return
	}
}
