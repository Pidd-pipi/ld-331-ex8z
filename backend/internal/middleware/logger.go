package middleware

import (
	"github.com/gin-gonic/gin"
	"log/slog"
	"time"
)

func RequestLogger(l *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = start.Format("20060102150405.000000000")
		}
		c.Header("X-Request-ID", rid)
		c.Next()
		l.Info("request", "request_id", rid, "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "latency_ms", time.Since(start).Milliseconds())
	}
}
