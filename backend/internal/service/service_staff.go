package service

import (
	"fmt"
	"github.com/gbsched/hospital-scheduler/internal/model"
	"github.com/gbsched/hospital-scheduler/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"log/slog"
)

type StaffService struct {
	repo   *repository.StaffRepository
	logger *slog.Logger
}

func NewStaffService(r *repository.StaffRepository, l *slog.Logger) *StaffService {
	return &StaffService{r, l}
}
func (s *StaffService) List(dept uint) ([]model.Staff, error) {
	v, e := s.repo.List(dept)
	if e != nil {
		return nil, fmt.Errorf("list staff: %w", e)
	}
	return v, nil
}
func (s *StaffService) Create(v model.Staff, password string) error {
	h, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return fmt.Errorf("hash password: %w", e)
	}
	v.PasswordHash = string(h)
	v.Active = true
	if e = s.repo.Create(&v); e != nil {
		return fmt.Errorf("create staff: %w", e)
	}
	return nil
}
func (s *StaffService) Authenticate(username, password string) (model.Staff, error) {
	v, e := s.repo.ByUsername(username)
	if e != nil {
		return v, fmt.Errorf("find login user: %w", e)
	}
	if !v.Active || bcrypt.CompareHashAndPassword([]byte(v.PasswordHash), []byte(password)) != nil {
		return v, fmt.Errorf("invalid credentials")
	}
	return v, nil
}
