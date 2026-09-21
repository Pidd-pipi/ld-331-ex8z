package service

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 每个用例使用独立的共享缓存内存库，并限制单连接，避免 :memory: 多连接各自建库。
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, e := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	sqlDB, e := db.DB()
	if e != nil {
		t.Fatal(e)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func newTestService(t *testing.T) (*ScheduleService, *gorm.DB, []model.Staff, []model.Shift, model.Department) {
	t.Helper()
	db := testDB(t)
	if e := db.AutoMigrate(&model.Department{}, &model.Position{}, &model.Staff{}, &model.Shift{}, &model.Schedule{}, &model.ScheduleRule{}, &model.Holiday{}, &model.ShiftRequest{}); e != nil {
		t.Fatal(e)
	}
	dept := model.Department{Name: "内科"}
	db.Create(&dept)
	var staffs []model.Staff
	for _, name := range []string{"A", "B", "C", "D"} {
		st := model.Staff{Name: name, Username: name + "user", Active: true, DepartmentID: dept.ID}
		db.Create(&st)
		staffs = append(staffs, st)
	}
	var shifts []model.Shift
	for _, sh := range []model.Shift{
		{Name: "day", Kind: model.ShiftDay},
		{Name: "evening", Kind: model.ShiftEvening},
		{Name: "night", Kind: model.ShiftNight},
		{Name: "rest", Kind: model.ShiftRest},
	} {
		db.Create(&sh)
		shifts = append(shifts, sh)
	}
	svc := NewScheduleService(db, repository.NewScheduleRepository(db), repository.NewStaffRepository(db), repository.NewShiftRepository(db), repository.NewScheduleRuleRepository(db), repository.NewHolidayRepository(db), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, db, staffs, shifts, dept
}

func shiftByKind(t *testing.T, shifts []model.Shift, kind model.ShiftKind) model.Shift {
	t.Helper()
	for _, sh := range shifts {
		if sh.Kind == kind {
			return sh
		}
	}
	t.Fatalf("shift kind %s not seeded", kind)
	return model.Shift{}
}

func TestGenerateSchedule(t *testing.T) {
	svc, _, staffs, _, dept := newTestService(t)
	n, e := svc.Generate(dept.ID, time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 8, 3, 0, 0, 0, 0, time.Local))
	if e != nil {
		t.Fatalf("generate: %v", e)
	}
	if want := len(staffs) * 3; n != want {
		t.Fatalf("generate n=%d want %d", n, want)
	}
}

// 自动生成必须遵守连续工作上限：上限为 N 时不应出现超过 N 天的连续工作段。
func TestGenerateRespectsMaxConsecutiveDays(t *testing.T) {
	svc, db, staffs, _, dept := newTestService(t)
	if e := db.Create(&model.ScheduleRule{DepartmentID: dept.ID, MaxConsecutiveDays: 2, WeekendRotation: true, HolidayPriority: true, ForbidNightToDay: true}).Error; e != nil {
		t.Fatal(e)
	}
	from, to := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 8, 10, 0, 0, 0, 0, time.Local)
	if _, e := svc.Generate(dept.ID, from, to); e != nil {
		t.Fatalf("generate: %v", e)
	}
	rows, e := svc.List(dept.ID, 0, from, to)
	if e != nil {
		t.Fatal(e)
	}
	grid := map[uint]map[string]model.ShiftKind{}
	for _, r := range rows {
		if grid[r.StaffID] == nil {
			grid[r.StaffID] = map[string]model.ShiftKind{}
		}
		grid[r.StaffID][r.WorkDate.Format("2006-01-02")] = r.Shift.Kind
	}
	for _, st := range staffs {
		streak := 0
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			if grid[st.ID][d.Format("2006-01-02")] != model.ShiftRest {
				streak++
				if streak > 2 {
					t.Fatalf("staff %s worked %d consecutive days, limit 2", st.Name, streak)
				}
			} else {
				streak = 0
			}
		}
	}
}

// 自动生成必须遵守“夜班后不得排白班”，且节假日默认只保留最少值班。
func TestGenerateNightRestAndHolidayPriority(t *testing.T) {
	svc, db, staffs, shifts, dept := newTestService(t)
	if e := db.Create(&model.ScheduleRule{DepartmentID: dept.ID, MaxConsecutiveDays: 5, WeekendRotation: true, HolidayPriority: true, ForbidNightToDay: true}).Error; e != nil {
		t.Fatal(e)
	}
	holiday := time.Date(2026, 8, 5, 0, 0, 0, 0, time.Local)
	if e := db.Create(&model.Holiday{Date: holiday, Name: "测试节", IsWorkday: false, Multiplier: 2}).Error; e != nil {
		t.Fatal(e)
	}
	from, to := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local), time.Date(2026, 8, 10, 0, 0, 0, 0, time.Local)
	if _, e := svc.Generate(dept.ID, from, to); e != nil {
		t.Fatalf("generate: %v", e)
	}
	rows, e := svc.List(dept.ID, 0, from, to)
	if e != nil {
		t.Fatal(e)
	}
	night := shiftByKind(t, shifts, model.ShiftNight)
	workingOnHoliday := 0
	byStaffDate := map[uint]map[string]model.Schedule{}
	for _, r := range rows {
		if byStaffDate[r.StaffID] == nil {
			byStaffDate[r.StaffID] = map[string]model.Schedule{}
		}
		byStaffDate[r.StaffID][r.WorkDate.Format("2006-01-02")] = r
		if r.WorkDate.Format("2006-01-02") == holiday.Format("2006-01-02") && r.Shift.Kind != model.ShiftRest {
			workingOnHoliday++
		}
	}
	for _, st := range staffs {
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			cur := byStaffDate[st.ID][d.Format("2006-01-02")]
			next, exists := byStaffDate[st.ID][d.AddDate(0, 0, 1).Format("2006-01-02")]
			if exists && cur.ShiftID == night.ID && next.Shift.Kind == model.ShiftDay {
				t.Fatalf("staff %s has day shift right after night on %s", st.Name, d.Format("2006-01-02"))
			}
		}
	}
	if workingOnHoliday > 1 {
		t.Fatalf("expected at most 1 staff working on holiday, got %d", workingOnHoliday)
	}
}

// 手动调整制造“夜班后白班”必须整次拒绝，原班表保持不变。
func TestUpdateRejectsNightToDayConflict(t *testing.T) {
	svc, db, staffs, shifts, dept := newTestService(t)
	if e := db.Create(&model.ScheduleRule{DepartmentID: dept.ID, MaxConsecutiveDays: 5, ForbidNightToDay: true}).Error; e != nil {
		t.Fatal(e)
	}
	day := shiftByKind(t, shifts, model.ShiftDay)
	night := shiftByKind(t, shifts, model.ShiftNight)
	rest := shiftByKind(t, shifts, model.ShiftRest)
	d1 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	d2 := d1.AddDate(0, 0, 1)
	first := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[0].ID, ShiftID: night.ID, WorkDate: d1}
	second := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[0].ID, ShiftID: rest.ID, WorkDate: d2}
	if e := db.Create(&first).Error; e != nil {
		t.Fatal(e)
	}
	if e := db.Create(&second).Error; e != nil {
		t.Fatal(e)
	}
	// 无冲突基线：夜班次日保持休息，复核通过。
	if e := svc.Update(second.ID, rest.ID, ""); e != nil {
		t.Fatalf("unexpected conflict for unchanged rest: %v", e)
	}
	// 把次日改成白班即触发“夜班后不得排白班”。
	err := svc.Update(second.ID, day.ID, "")
	if !errors.Is(err, ErrScheduleConflict) {
		t.Fatalf("want ErrScheduleConflict, got %v", err)
	}
	var reloaded model.Schedule
	if e := db.First(&reloaded, second.ID).Error; e != nil {
		t.Fatal(e)
	}
	if reloaded.ShiftID != rest.ID {
		t.Fatalf("original schedule changed after rejected update: %+v", reloaded)
	}
}

// 无冲突的手动调整应成功并清除遗留冲突标记。
func TestUpdateClearsResolvedConflict(t *testing.T) {
	svc, db, staffs, shifts, dept := newTestService(t)
	if e := db.Create(&model.ScheduleRule{DepartmentID: dept.ID, MaxConsecutiveDays: 5, ForbidNightToDay: true}).Error; e != nil {
		t.Fatal(e)
	}
	night := shiftByKind(t, shifts, model.ShiftNight)
	rest := shiftByKind(t, shifts, model.ShiftRest)
	d1 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	row := model.Schedule{DepartmentID: dept.ID, StaffID: staffs[0].ID, ShiftID: night.ID, WorkDate: d1, HasConflict: true, ConflictReasons: "遗留冲突"}
	if e := db.Create(&row).Error; e != nil {
		t.Fatal(e)
	}
	if e := svc.Update(row.ID, rest.ID, ""); e != nil {
		t.Fatalf("update to rest: %v", e)
	}
	var reloaded model.Schedule
	if e := db.First(&reloaded, row.ID).Error; e != nil {
		t.Fatal(e)
	}
	if reloaded.HasConflict || reloaded.ConflictReasons != "" {
		t.Fatalf("conflict flags not cleared: %+v", reloaded)
	}
}
