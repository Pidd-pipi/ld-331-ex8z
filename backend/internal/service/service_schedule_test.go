package service

import (
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestGenerateSchedule(t *testing.T) {
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if e = db.AutoMigrate(&model.Staff{}, &model.Shift{}, &model.Schedule{}, &model.ScheduleRule{}); e != nil {
		t.Fatal(e)
	}
	db.Create(&model.Staff{Name: "A", Username: "auser", Active: true, DepartmentID: 1})
	db.Create(&model.Shift{Name: "day", Kind: model.ShiftDay})
	db.Create(&model.Shift{Name: "night", Kind: model.ShiftNight})
	db.Create(&model.Shift{Name: "rest", Kind: model.ShiftRest})
	s := NewScheduleService(repository.NewScheduleRepository(db), repository.NewStaffRepository(db), repository.NewShiftRepository(db), repository.NewScheduleRuleRepository(db), slog.New(slog.NewTextHandler(io.Discard, nil)))
	n, e := s.Generate(1, time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 8, 3, 0, 0, 0, 0, time.Local))
	if e != nil || n != 3 {
		t.Fatalf("generate n=%d err=%v", n, e)
	}
}
