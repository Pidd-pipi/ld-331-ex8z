package router

import (
	"github.com/gbsched/hospital-scheduler/internal/handler"
	"github.com/gbsched/hospital-scheduler/internal/middleware"
	"github.com/gin-gonic/gin"
	"log/slog"
)

type Handlers struct {
	Auth        *handler.AuthHandler
	Departments *handler.DepartmentHandler
	Positions   *handler.PositionHandler
	Staff       *handler.StaffHandler
	Schedules   *handler.ScheduleHandler
	Requests    *handler.ShiftRequestHandler
	Rules       *handler.RuleHandler
	Holidays    *handler.HolidayHandler
	Audit       *handler.AuditHandler
}

func New(h Handlers, secret string, l *slog.Logger) *gin.Engine {
	g := gin.New()
	g.Use(gin.Recovery(), middleware.RequestLogger(l))
	g.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{"status": "healthy"}})
	})
	api := g.Group("/api/v1")
	registerAPI(api, h, secret)
	// Nginx strips /api/ when proxy_pass ends with a slash, so retain /v1 compatibility.
	proxyAPI := g.Group("/v1")
	registerAPI(proxyAPI, h, secret)
	return g
}

func registerAPI(api *gin.RouterGroup, h Handlers, secret string) {
	api.POST("/auth/login", h.Auth.Login)
	protected := api.Group("")
	protected.Use(middleware.Auth(secret))
	protected.GET("/departments", h.Departments.List)
	protected.GET("/positions", h.Positions.List)
	protected.GET("/staff", h.Staff.List)
	protected.GET("/schedules", h.Schedules.List)
	protected.GET("/schedules/statistics", h.Schedules.Stats)
	protected.GET("/schedules/export", h.Schedules.Export)
	protected.GET("/shift-requests", h.Requests.List)
	protected.POST("/shift-requests", h.Requests.Create)
	protected.GET("/holidays", h.Holidays.List)
	admin := protected.Group("")
	admin.Use(middleware.Roles("admin", "supervisor"))
	admin.POST("/departments", h.Departments.Create)
	admin.POST("/positions", h.Positions.Create)
	admin.POST("/staff", h.Staff.Create)
	admin.POST("/schedules/generate", h.Schedules.Generate)
	admin.PUT("/schedules/:id", h.Schedules.Update)
	admin.GET("/schedule-rules", h.Rules.Get)
	admin.PUT("/schedule-rules", h.Rules.Save)
	admin.PUT("/shift-requests/:id/review", h.Requests.Review)
	admin.POST("/holidays", h.Holidays.Create)
	admin.GET("/audit-logs", h.Audit.List)
}
