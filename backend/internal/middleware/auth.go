package middleware

import (
	"github.com/gbsched/hospital-scheduler/internal/constants"
	"github.com/gbsched/hospital-scheduler/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
)

type Claims struct {
	StaffID uint   `json:"staff_id"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			response.Error(c, http.StatusUnauthorized, constants.CodeUnauthorized, "缺少认证令牌")
			c.Abort()
			return
		}
		t, e := jwt.ParseWithClaims(strings.TrimPrefix(h, "Bearer "), &Claims{}, func(token *jwt.Token) (any, error) { return []byte(secret), nil })
		if e != nil || !t.Valid {
			response.Error(c, http.StatusUnauthorized, constants.CodeUnauthorized, "认证令牌无效")
			c.Abort()
			return
		}
		claims, ok := t.Claims.(*Claims)
		if !ok {
			response.Error(c, http.StatusUnauthorized, constants.CodeUnauthorized, "认证令牌无效")
			c.Abort()
			return
		}
		c.Set("staff_id", claims.StaffID)
		c.Set("role", claims.Role)
		c.Next()
	}
}
func Roles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		for _, v := range roles {
			if role == v {
				c.Next()
				return
			}
		}
		response.Error(c, http.StatusForbidden, constants.CodeForbidden, "无权限执行该操作")
		c.Abort()
	}
}
func StaffID(c *gin.Context) uint { v, _ := c.Get("staff_id"); id, _ := v.(uint); return id }
