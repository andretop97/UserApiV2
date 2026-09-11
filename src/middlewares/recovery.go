package middlewares

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

func SlogRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error(
					"Panic recovered",
					slog.Any("error", err),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
					slog.String("query", c.Request.URL.RawQuery),
					slog.String("ip", c.ClientIP()),
					slog.String("user-agent", c.Request.UserAgent()),
					slog.String("stack", string(debug.Stack())),
				)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
