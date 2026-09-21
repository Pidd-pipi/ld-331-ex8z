package service

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"gorm.io/gorm"
)

func newRequestFixture(t *testing.T) (*ShiftRequestService, *gorm.DB, *ScheduleService, []model.Staff, []model.Shift, model.Department) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := testDB(t)
	if e := db.AutoMigrate(&model.Department{}, &model.Staff{}, &model.Shift{}, &model.Schedule{}, &model.ScheduleRule{}, &model.Holiday{}, &model.ShiftRequest{}); e != nil {
		t.Fatal(e)
	}
	dept := model.Department{Name: "急诊科"}
	db.Create(&dept)
	var staffs []model.Staff
	for _, name := range []string{"A", "B"} {
		st := model.Staff{Name: name, Username: "req" + name, Active: true, DepartmentID: dept.ID}
		db.Create(&st)
		staffs = append(staffs, st)
	}
	var shifts []model.Shift
	for _, sh := range []model.Shift{
		{Name: "day", Kind: model.ShiftDay},
		{Name: "night", Kind: model.ShiftNight},
		{Name: "rest", Kind: model.ShiftRest},
	} {
		db.Create(&sh)
		shifts = append(shifts, sh)
	}
	schedRepo := repository.NewScheduleRepository(db)
	schedSvc := NewScheduleService(db, schedRepo, repository.NewStaffRepository(db), repository.NewShiftRepository(db), repository.NewScheduleRuleRepository(db), repository.NewHolidayRepository(db), logger)
	reqSvc := NewShiftRequestService(repository.NewShiftRequestRepository(db), schedRepo, schedSvc, db, logger)
	return reqSvc, db, schedSvc, staffs, shifts, dept
}

// 审批通过交换排班时若产生“夜班后白班”，必须整次拒绝：班表不动、申请仍为 pending。
func TestReviewApprovalRejectedOnConflict(t *testing.T) {
	reqSvc, db, _, staffs, shifts, dept := newRequestFixture(t)
	day, night, rest := shifts[0], shifts[1], shifts[2]
	if e := db.Create(&model.ScheduleRule{DepartmentID: dept.ID, MaxConsecutiveDays: 5, ForbidNightToDay: true}).Error; e != nil {
		t.Fatal(e)
	}
	d1 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	d2 := d1.AddDate(0, 0, 1)
	// A：d1 夜班、d2 白班（申请行 a）。B：d1 夜班、d2 休息（申请行 b）。
	aNight := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[0].ID, ShiftID: night.ID, WorkDate: d1}
	bNight := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[1].ID, ShiftID: night.ID, WorkDate: d1}
	aDay := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[0].ID, ShiftID: day.ID, WorkDate: d2}
	bRest := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[1].ID, ShiftID: rest.ID, WorkDate: d2}
	for _, s := range []*model.Schedule{&aNight, &bNight, &aDay, &bRest} {
		if e := db.Create(s).Error; e != nil {
			t.Fatal(e)
		}
	}
	req := model.ShiftRequest{ApplicantID: staffs[0].ID, SubstituteID: staffs[1].ID, ScheduleID: aDay.ID, SubstituteScheduleID: bRest.ID, Reason: "家中有事", Status: model.RequestPending}
	if e := db.Create(&req).Error; e != nil {
		t.Fatal(e)
	}
	// 交换后 B 在 d2 上白班，而 B 的 d1 是夜班 → 冲突。
	err := reqSvc.Review(req.ID, staffs[0].ID, true)
	if !errors.Is(err, ErrScheduleConflict) {
		t.Fatalf("want ErrScheduleConflict, got %v", err)
	}
	var pending model.ShiftRequest
	if e := db.First(&pending, req.ID).Error; e != nil {
		t.Fatal(e)
	}
	if pending.Status != model.RequestPending || pending.ReviewerID != nil {
		t.Fatalf("request state changed after conflict: %+v", pending)
	}
	var rowB model.Schedule
	if e := db.First(&rowB, bRest.ID).Error; e != nil {
		t.Fatal(e)
	}
	if rowB.StaffID != staffs[1].ID || rowB.ShiftID != rest.ID {
		t.Fatalf("original schedule changed after rejected approval: %+v", rowB)
	}
}

// 无冲突交换应正常审批，双方班表在同一事务中交换且申请变为 approved。
func TestReviewApprovalSucceeds(t *testing.T) {
	reqSvc, db, _, staffs, shifts, dept := newRequestFixture(t)
	day, rest := shifts[0], shifts[2]
	if e := db.Create(&model.ScheduleRule{DepartmentID: dept.ID, MaxConsecutiveDays: 5, ForbidNightToDay: true}).Error; e != nil {
		t.Fatal(e)
	}
	d := time.Date(2026, 9, 3, 0, 0, 0, 0, time.Local)
	aDay := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[0].ID, ShiftID: day.ID, WorkDate: d}
	bRest := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[1].ID, ShiftID: rest.ID, WorkDate: d}
	if e := db.Create(&aDay).Error; e != nil {
		t.Fatal(e)
	}
	if e := db.Create(&bRest).Error; e != nil {
		t.Fatal(e)
	}
	req := model.ShiftRequest{ApplicantID: staffs[0].ID, SubstituteID: staffs[1].ID, ScheduleID: aDay.ID, SubstituteScheduleID: bRest.ID, Reason: "需要调班", Status: model.RequestPending}
	if e := db.Create(&req).Error; e != nil {
		t.Fatal(e)
	}
	if e := reqSvc.Review(req.ID, 99, true); e != nil {
		t.Fatalf("approve: %v", e)
	}
	var approved model.ShiftRequest
	if e := db.First(&approved, req.ID).Error; e != nil {
		t.Fatal(e)
	}
	if approved.Status != model.RequestApproved {
		t.Fatalf("status = %s", approved.Status)
	}
	var r1, r2 model.Schedule
	db.First(&r1, aDay.ID)
	db.First(&r2, bRest.ID)
	if r1.StaffID != staffs[1].ID || r2.StaffID != staffs[0].ID {
		t.Fatalf("schedules not swapped: %+v %+v", r1, r2)
	}
}

// 交换前遗留的冲突标记应在审批通过后按两位原班主清除（验证不是按交换后的归属清除）。
func TestReviewApprovalClearsLegacyConflictFlags(t *testing.T) {
	reqSvc, db, _, staffs, shifts, dept := newRequestFixture(t)
	day, rest := shifts[0], shifts[2]
	if e := db.Create(&model.ScheduleRule{DepartmentID: dept.ID, MaxConsecutiveDays: 5, ForbidNightToDay: false, HolidayPriority: false}).Error; e != nil {
		t.Fatal(e)
	}
	d := time.Date(2026, 9, 5, 0, 0, 0, 0, time.Local)
	aDay := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[0].ID, ShiftID: day.ID, WorkDate: d, HasConflict: true, ConflictReasons: "旧冲突A"}
	bRest := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[1].ID, ShiftID: rest.ID, WorkDate: d, HasConflict: true, ConflictReasons: "旧冲突B"}
	if e := db.Create(&aDay).Error; e != nil {
		t.Fatal(e)
	}
	if e := db.Create(&bRest).Error; e != nil {
		t.Fatal(e)
	}
	req := model.ShiftRequest{ApplicantID: staffs[0].ID, SubstituteID: staffs[1].ID, ScheduleID: aDay.ID, SubstituteScheduleID: bRest.ID, Reason: "需要调班", Status: model.RequestPending}
	if e := db.Create(&req).Error; e != nil {
		t.Fatal(e)
	}
	if e := reqSvc.Review(req.ID, 1, true); e != nil {
		t.Fatalf("approve: %v", e)
	}
	for _, id := range []uint{aDay.ID, bRest.ID} {
		var row model.Schedule
		if e := db.First(&row, id).Error; e != nil {
			t.Fatal(e)
		}
		if row.HasConflict || row.ConflictReasons != "" {
			t.Fatalf("legacy conflict flag not cleared on row %d: %+v", id, row)
		}
	}
}
