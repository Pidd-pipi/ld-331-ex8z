package service

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&model.Department{}, &model.Staff{}, &model.Shift{}, &model.Schedule{}, &model.ScheduleRule{}, &model.Holiday{}, &model.ShiftRequest{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func testServices(t *testing.T) (*gorm.DB, *ScheduleService, *ShiftRequestService) {
	t.Helper()
	db := newTestDB(t)
	schedRepo := repository.NewScheduleRepository(db)
	staffRepo := repository.NewStaffRepository(db)
	shiftRepo := repository.NewShiftRepository(db)
	ruleRepo := repository.NewScheduleRuleRepository(db)
	holidayRepo := repository.NewHolidayRepository(db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, sh := range []model.Shift{
		{Name: "白班", Kind: model.ShiftDay},
		{Name: "中班", Kind: model.ShiftEvening},
		{Name: "夜班", Kind: model.ShiftNight},
		{Name: "休息", Kind: model.ShiftRest},
	} {
		if err := db.Create(&sh).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"auser", "buser"} {
		if err := db.Create(&model.Staff{Name: name, Username: name, Active: true, DepartmentID: 1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := ruleRepo.Upsert(&model.ScheduleRule{DepartmentID: 1, MaxConsecutiveDays: 5, HolidayPriority: true, ForbidNightToDay: true}); err != nil {
		t.Fatal(err)
	}
	engine := NewRuleEngine(schedRepo, ruleRepo, holidayRepo)
	schedSvc := NewScheduleService(schedRepo, staffRepo, shiftRepo, ruleRepo, holidayRepo, engine, logger)
	reqSvc := NewShiftRequestService(repository.NewShiftRequestRepository(db), schedRepo, shiftRepo, engine, db, logger)
	return db, schedSvc, reqSvc
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

func TestGenerateSchedule(t *testing.T) {
	_, svc, _ := testServices(t)
	from, to := day(2026, 8, 1), day(2026, 8, 3)
	n, err := svc.Generate(1, from, to)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if n != 6 { // 2 staff * 3 days
		t.Fatalf("generated rows = %d, want 6", n)
	}
}

// Generated schedules must obey consecutive-work cap and night-to-day rules.
func TestGenerateRulesCompliant(t *testing.T) {
	db, svc, _ := testServices(t)
	from, to := day(2026, 8, 1), day(2026, 8, 21) // 3 weeks
	if _, err := svc.Generate(1, from, to); err != nil {
		t.Fatalf("generate: %v", err)
	}
	var rows []model.Schedule
	if err := db.Preload("Shift").Order("work_date,staff_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	byStaff := map[uint][]model.Schedule{}
	for _, r := range rows {
		byStaff[r.StaffID] = append(byStaff[r.StaffID], r)
	}
	for staff, list := range byStaff {
		streak := 0
		for i, r := range list {
			if r.Shift.Kind == model.ShiftRest {
				streak = 0
				continue
			}
			streak++
			if streak > 5 {
				t.Fatalf("staff %d streak %d > 5 on %s", staff, streak, r.WorkDate.Format("2006-01-02"))
			}
			if i > 0 && list[i-1].Shift.Kind == model.ShiftNight && r.Shift.Kind == model.ShiftDay {
				t.Fatalf("staff %d day shift after night on %s", staff, r.WorkDate.Format("2006-01-02"))
			}
			if r.ConflictReason != "" {
				t.Fatalf("staff %d has conflict on %s: %s", staff, r.WorkDate.Format("2006-01-02"), r.ConflictReason)
			}
		}
	}
}

// On a holiday, the member with fewer holiday rests must rest; the working
// member must not be flagged because the rotation is fair.
func TestGenerateHolidayPriority(t *testing.T) {
	db, svc, _ := testServices(t)
	holidayRepo := repository.NewHolidayRepository(db)
	if err := holidayRepo.Create(&model.Holiday{Date: day(2026, 10, 1), Name: "国庆节", Multiplier: 3}); err != nil {
		t.Fatal(err)
	}
	from, to := day(2026, 9, 30), day(2026, 10, 2)
	if _, err := svc.Generate(1, from, to); err != nil {
		t.Fatalf("generate: %v", err)
	}
	var holidayRows []model.Schedule
	if err := db.Preload("Shift").Where("work_date >= ? AND work_date <= ?", day(2026, 10, 1), day(2026, 10, 1)).Find(&holidayRows).Error; err != nil {
		t.Fatal(err)
	}
	rests, workers := 0, 0
	for _, r := range holidayRows {
		switch r.Shift.Kind {
		case model.ShiftRest:
			rests++
			if r.ConflictReason != "" {
				t.Fatalf("resting member flagged: %s", r.ConflictReason)
			}
		default:
			workers++
			if r.ConflictReason != "" {
				t.Fatalf("working member flagged on first holiday: %s", r.ConflictReason)
			}
		}
	}
	if rests != 1 || workers != 1 {
		t.Fatalf("expected 1 rest / 1 worker, got %d rest / %d worker", rests, workers)
	}
}

// When minimum coverage forces a member to work on a day the rules require a
// rest, the unavoidable violation must be recorded on the member's row.
func TestGenerateRecordsUnavoidableConflict(t *testing.T) {
	db, svc, _ := testServices(t)
	ruleRepo := repository.NewScheduleRuleRepository(db)
	// With two staff, max consecutive = 1 and night-to-day enforced, the two
	// rest cycles eventually overlap; coverage then pulls someone back in.
	if err := ruleRepo.Upsert(&model.ScheduleRule{DepartmentID: 1, MaxConsecutiveDays: 1, ForbidNightToDay: true, HolidayPriority: false}); err != nil {
		t.Fatal(err)
	}
	from, to := day(2026, 8, 1), day(2026, 8, 10)
	if _, err := svc.Generate(1, from, to); err != nil {
		t.Fatalf("generate: %v", err)
	}
	var flagged []model.Schedule
	if err := db.Where("conflict_reason <> ''").Find(&flagged).Error; err != nil {
		t.Fatal(err)
	}
	if len(flagged) == 0 {
		t.Fatal("expected unavoidable coverage conflicts to be recorded")
	}
	for _, f := range flagged {
		if f.ConflictReason == "" {
			t.Fatal("flagged row has empty reason")
		}
		// Every working day must still be covered.
		var workers int64
		db.Model(&model.Schedule{}).Joins("JOIN shifts ON shifts.id = schedules.shift_id").
			Where("work_date = ? AND shifts.kind <> ?", f.WorkDate, model.ShiftRest).Count(&workers)
		if workers == 0 {
			t.Fatalf("no coverage on %s", f.WorkDate.Format("2006-01-02"))
		}
	}
	t.Logf("recorded %d unavoidable conflict rows", len(flagged))
}

// Manual update that creates a day-shift-right-after-night must be rejected
// and leave the original row untouched.
func TestManualUpdateRejectsConflict(t *testing.T) {
	db, svc, _ := testServices(t)
	from, to := day(2026, 8, 1), day(2026, 8, 14)
	if _, err := svc.Generate(1, from, to); err != nil {
		t.Fatalf("generate: %v", err)
	}
	shiftRepo := repository.NewShiftRepository(db)
	dayShift, _ := shiftRepo.ByKind(model.ShiftDay)

	// Find any night shift and force its staff onto a day shift next day.
	var night model.Schedule
	if err := db.Preload("Shift").Joins("JOIN shifts ON shifts.id = schedules.shift_id").Where("shifts.kind = ?", model.ShiftNight).First(&night).Error; err != nil {
		t.Skip("no night shift generated in range")
	}
	var next model.Schedule
	if err := db.Preload("Shift").Where("staff_id = ? AND work_date = ?", night.StaffID, night.WorkDate.AddDate(0, 0, 1)).First(&next).Error; err != nil {
		t.Fatal(err)
	}
	originalShift := next.ShiftID
	err := svc.Update(next.ID, dayShift.ID, "")
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	var ce *ScheduleConflictError
	if !asConflict(err, &ce) {
		t.Fatalf("expected ScheduleConflictError, got %T %v", err, err)
	}
	var reloaded model.Schedule
	if err := db.First(&reloaded, next.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.ShiftID != originalShift {
		t.Fatalf("original schedule mutated: %d -> %d", originalShift, reloaded.ShiftID)
	}
}

// A compliant manual update is accepted and clears stale conflict reasons.
func TestManualUpdateAcceptsCompliant(t *testing.T) {
	db, svc, _ := testServices(t)
	from, to := day(2026, 8, 1), day(2026, 8, 7)
	if _, err := svc.Generate(1, from, to); err != nil {
		t.Fatalf("generate: %v", err)
	}
	shiftRepo := repository.NewShiftRepository(db)
	rest, _ := shiftRepo.ByKind(model.ShiftRest)
	var target model.Schedule
	if err := db.First(&target, "work_date = ?", day(2026, 8, 2)).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(target.ID, rest.ID, ""); err != nil {
		t.Fatalf("expected compliant rest update, got %v", err)
	}
}

// Turning a rest day into work several days before an already full streak
// must also be rejected (the conflict may land beyond the edited day).
func TestManualUpdateRejectsDelayedStreak(t *testing.T) {
	db, svc, _ := testServices(t)
	if err := repository.NewScheduleRuleRepository(db).Upsert(&model.ScheduleRule{DepartmentID: 1, MaxConsecutiveDays: 3, ForbidNightToDay: false, HolidayPriority: false}); err != nil {
		t.Fatal(err)
	}
	shiftRepo := repository.NewShiftRepository(db)
	work, _ := shiftRepo.ByKind(model.ShiftDay)
	rest, _ := shiftRepo.ByKind(model.ShiftRest)
	// Build 8 days manually: staff1 rests only on day 4, otherwise works.
	dates := []time.Time{}
	for i := 0; i < 8; i++ {
		dates = append(dates, day(2026, 8, 1+i))
	}
	for i, d := range dates {
		sh := work.ID
		if i == 3 {
			sh = rest.ID
		}
		if err := db.Create(&model.Schedule{DepartmentID: 1, StaffID: 1, ShiftID: sh, WorkDate: d}).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Fill staff2 rows so the engine has complete data.
	for _, d := range dates {
		if err := db.Create(&model.Schedule{DepartmentID: 1, StaffID: 2, ShiftID: rest.ID, WorkDate: d}).Error; err != nil {
			t.Fatal(err)
		}
	}
	var restRow model.Schedule
	if err := db.First(&restRow, "work_date = ? AND staff_id = ?", day(2026, 8, 4), 1).Error; err != nil {
		t.Fatal(err)
	}
	// Making day 4 a work day yields an unbroken 8-day streak -> cap breach.
	if err := svc.Update(restRow.ID, work.ID, ""); err == nil {
		t.Fatal("expected delayed-streak conflict rejection")
	}
	var reloaded model.Schedule
	db.First(&reloaded, restRow.ID)
	if reloaded.ShiftID != rest.ID {
		t.Fatalf("rest row mutated to shift %d", reloaded.ShiftID)
	}
}

// Approving a swap that creates a night->day transition leaves status pending
// and schedules unchanged.
func TestReviewRejectsConflictingSwap(t *testing.T) {
	db, _, reqSvc := testServices(t)
	shiftRepo := repository.NewShiftRepository(db)
	dayShift, _ := shiftRepo.ByKind(model.ShiftDay)
	nightShift, _ := shiftRepo.ByKind(model.ShiftNight)
	rest, _ := shiftRepo.ByKind(model.ShiftRest)

	// staff1 keeps a night on 8/1 and offers a rest on 8/3; staff2 works a
	// day on 8/2. Swapping the rest with that day puts staff1 on a day
	// immediately after their night -> conflict.
	r1 := model.Schedule{DepartmentID: 1, StaffID: 1, ShiftID: nightShift.ID, WorkDate: day(2026, 8, 1)}
	r0 := model.Schedule{DepartmentID: 1, StaffID: 1, ShiftID: rest.ID, WorkDate: day(2026, 8, 3)}
	r2 := model.Schedule{DepartmentID: 1, StaffID: 2, ShiftID: dayShift.ID, WorkDate: day(2026, 8, 2)}
	for _, s := range []*model.Schedule{&r1, &r0, &r2} {
		if err := db.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	req := model.ShiftRequest{ApplicantID: 1, SubstituteID: 2, ScheduleID: r0.ID, SubstituteScheduleID: r2.ID, Reason: "家中有事需要调班", Status: model.RequestPending}
	if err := db.Create(&req).Error; err != nil {
		t.Fatal(err)
	}
	err := reqSvc.Review(req.ID, 9, true)
	if err == nil {
		t.Fatal("expected conflict rejection")
	}
	var ce *ScheduleConflictError
	if !asConflict(err, &ce) {
		t.Fatalf("expected ScheduleConflictError, got %v", err)
	}
	var reloaded model.ShiftRequest
	if err := db.First(&reloaded, req.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != model.RequestPending {
		t.Fatalf("request status changed to %s", reloaded.Status)
	}
	var a, b model.Schedule
	db.First(&a, r0.ID)
	db.First(&b, r2.ID)
	if a.StaffID != 1 || b.StaffID != 2 {
		t.Fatal("schedules mutated despite conflict rejection")
	}
}

// A clean swap (rests) is approved atomically and flips both staff ids.
func TestReviewApprovesCleanSwap(t *testing.T) {
	db, _, reqSvc := testServices(t)
	rest, _ := repository.NewShiftRepository(db).ByKind(model.ShiftRest)
	a := model.Schedule{DepartmentID: 1, StaffID: 1, ShiftID: rest.ID, WorkDate: day(2026, 8, 3)}
	b := model.Schedule{DepartmentID: 1, StaffID: 2, ShiftID: rest.ID, WorkDate: day(2026, 8, 4)}
	db.Create(&a)
	db.Create(&b)
	req := model.ShiftRequest{ApplicantID: 1, SubstituteID: 2, ScheduleID: a.ID, SubstituteScheduleID: b.ID, Reason: "互相同意调换休息日", Status: model.RequestPending}
	db.Create(&req)
	if err := reqSvc.Review(req.ID, 9, true); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var ra, rb model.Schedule
	db.First(&ra, a.ID)
	db.First(&rb, b.ID)
	if ra.StaffID != 2 || rb.StaffID != 1 {
		t.Fatalf("swap not applied: %d/%d", ra.StaffID, rb.StaffID)
	}
	var rr model.ShiftRequest
	db.First(&rr, req.ID)
	if rr.Status != model.RequestApproved {
		t.Fatalf("status = %s", rr.Status)
	}
}

func asConflict(err error, target **ScheduleConflictError) bool {
	for err != nil {
		if c, ok := err.(*ScheduleConflictError); ok {
			*target = c
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
