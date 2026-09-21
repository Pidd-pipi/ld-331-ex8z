package handler

import (
	"github.com/gbsched/hospital-scheduler/internal/constants"
	"github.com/gbsched/hospital-scheduler/internal/dto"
	"github.com/gbsched/hospital-scheduler/internal/middleware"
	"github.com/gbsched/hospital-scheduler/internal/service"
	"github.com/gbsched/hospital-scheduler/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"time"
)

type AuthHandler struct {
	s      *service.StaffService
	secret string
}

func NewAuthHandler(s *service.StaffService, secret string) *AuthHandler {
	return &AuthHandler{s, secret}
}
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		response.Error(c, 400, constants.CodeBadRequest, "用户名或密码格式错误")
		return
	}
	u, e := h.s.Authenticate(req.Username, req.Password)
	if e != nil {
		response.Error(c, http.StatusUnauthorized, constants.CodeUnauthorized, "用户名或密码错误")
		return
	}
	token, e := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.Claims{StaffID: u.ID, Role: string(u.Role), RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour))}}).SignedString([]byte(h.secret))
	if e != nil {
		response.Error(c, 500, constants.CodeInternal, "签发令牌失败")
		return
	}
	response.OK(c, dto.LoginResponse{Token: token, Name: u.Name, Role: string(u.Role), StaffID: u.ID})
}
