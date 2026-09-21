package response

import "github.com/gin-gonic/gin"

type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func OK(c *gin.Context, data any) { c.JSON(200, Body{Code: 0, Message: "ok", Data: data}) }
func Error(c *gin.Context, status, code int, msg string) {
	c.JSON(status, Body{Code: code, Message: msg})
}
