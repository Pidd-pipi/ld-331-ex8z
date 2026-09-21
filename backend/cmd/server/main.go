package main

import (
	"fmt"
	"github.com/gbsched/hospital-scheduler/internal/config"
	"github.com/gbsched/hospital-scheduler/internal/handler"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"github.com/gbsched/hospital-scheduler/internal/router"
	"github.com/gbsched/hospital-scheduler/internal/service"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log/slog"
	"os"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	var dial gorm.Dialector
	if cfg.DBDriver == "mysql" {
		dial = mysql.Open(cfg.DSN())
	} else {
		dial = sqlite.Open(cfg.DSN())
	}
	db, e := gorm.Open(dial, &gorm.Config{})
	if e != nil {
		panic(fmt.Errorf("connect database: %w", e))
	}
	if e = db.AutoMigrate(&model.Department{}, &model.Position{}, &model.Staff{}, &model.Shift{}, &model.Schedule{}, &model.ShiftRequest{}, &model.ScheduleRule{}, &model.Holiday{}, &model.AuditLog{}); e != nil {
		panic(fmt.Errorf("migrate database: %w", e))
	}
	seed(db)
	depRepo := repository.NewDepartmentRepository(db)
	posRepo := repository.NewPositionRepository(db)
	staffRepo := repository.NewStaffRepository(db)
	shiftRepo := repository.NewShiftRepository(db)
	schedRepo := repository.NewScheduleRepository(db)
	reqRepo := repository.NewShiftRequestRepository(db)
	ruleRepo := repository.NewScheduleRuleRepository(db)
	holidayRepo := repository.NewHolidayRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)
	depSvc := service.NewDepartmentService(depRepo, logger)
	posSvc := service.NewPositionService(posRepo, logger)
	staffSvc := service.NewStaffService(staffRepo, logger)
	schedSvc := service.NewScheduleService(schedRepo, staffRepo, shiftRepo, ruleRepo, logger)
	reqSvc := service.NewShiftRequestService(reqRepo, schedRepo, db, logger)
	ruleSvc := service.NewRuleService(ruleRepo, logger)
	holidaySvc := service.NewHolidayService(holidayRepo, logger)
	auditSvc := service.NewAuditService(auditRepo, logger)
	r := router.New(router.Handlers{Auth: handler.NewAuthHandler(staffSvc, cfg.JWTSecret), Departments: handler.NewDepartmentHandler(depSvc), Positions: handler.NewPositionHandler(posSvc), Staff: handler.NewStaffHandler(staffSvc), Schedules: handler.NewScheduleHandler(schedSvc), Requests: handler.NewShiftRequestHandler(reqSvc), Rules: handler.NewRuleHandler(ruleSvc), Holidays: handler.NewHolidayHandler(holidaySvc), Audit: handler.NewAuditHandler(auditSvc)}, cfg.JWTSecret, logger)
	logger.Info("hospital scheduler started", "port", cfg.Port, "driver", cfg.DBDriver)
	if e := r.Run(":" + cfg.Port); e != nil {
		logger.Error("server stopped", "error", e)
	}
}
func seed(db *gorm.DB) {
	var count int64
	db.Model(&model.Department{}).Count(&count)
	if count > 0 {
		return
	}
	d := model.Department{Name: "内科"}
	db.Create(&d)
	p := model.Position{DepartmentID: d.ID, Name: "主治医师", StaffingQuota: 12, Skills: "内科,急诊"}
	db.Create(&p)
	for _, s := range []model.Shift{{Name: "白班", Kind: model.ShiftDay, Color: "#409EFF", StartTime: "08:00", EndTime: "16:00", Hours: 8}, {Name: "中班", Kind: model.ShiftEvening, Color: "#E6A23C", StartTime: "16:00", EndTime: "00:00", Hours: 8}, {Name: "夜班", Kind: model.ShiftNight, Color: "#722ED1", StartTime: "00:00", EndTime: "08:00", Hours: 8}, {Name: "休息", Kind: model.ShiftRest, Color: "#909399", StartTime: "00:00", EndTime: "00:00", Hours: 0}} {
		db.Create(&s)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	for _, u := range []model.Staff{{Name: "系统管理员", Username: "admin", PasswordHash: string(hash), Role: model.RoleAdmin, DepartmentID: d.ID, PositionID: p.ID, Active: true}, {Name: "李主管", Username: "supervisor", PasswordHash: string(hash), Role: model.RoleSupervisor, DepartmentID: d.ID, PositionID: p.ID, Active: true}, {Name: "王医生", Username: "doctor", PasswordHash: string(hash), Role: model.RoleStaff, DepartmentID: d.ID, PositionID: p.ID, Active: true}, {Name: "张护士", Username: "nurse", PasswordHash: string(hash), Role: model.RoleStaff, DepartmentID: d.ID, PositionID: p.ID, Active: true}} {
		db.Create(&u)
	}
	db.Create(&model.ScheduleRule{DepartmentID: d.ID, MaxConsecutiveDays: 5, WeekendRotation: true, HolidayPriority: true, ForbidNightToDay: true})
}
