package repository

import (
	"github.com/gbsched/hospital-scheduler/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestDepartmentRepositoryList(t *testing.T) {
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	db.AutoMigrate(&model.Department{}, &model.Position{})
	r := NewDepartmentRepository(db)
	if e = r.Create(&model.Department{Name: "急诊科"}); e != nil {
		t.Fatal(e)
	}
	v, e := r.List()
	if e != nil || len(v) != 1 {
		t.Fatalf("got %d %v", len(v), e)
	}
}
