package middlewares

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func SlogLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		attrs := []any{
			slog.Int("status", statusCode),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("query", query),
			slog.String("ip", c.ClientIP()),
			slog.String("user-agent", c.Request.UserAgent()),
			slog.Duration("latency", latency),
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		switch {
		case statusCode >= 500:
			slog.Error("Request completed with server error", attrs...)
		case statusCode >= 400:
			slog.Warn("Request completed with client error", attrs...)
		default:
			slog.Info("Request completed successfully", attrs...)
		}
	}
}
