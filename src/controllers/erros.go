package controllers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/gin-gonic/gin"
)

var errorStatus = map[error]int{
	core.ErrBadRequest:         http.StatusBadRequest,
	core.ErrUserNotFound:       http.StatusNotFound,
	core.ErrEmailAlreadyExists: http.StatusConflict,
	core.ErrInvalidCredentials: http.StatusUnauthorized,
}

func respondError(c *gin.Context, err error) {
	for domainErr, status := range errorStatus {
		if errors.Is(err, domainErr) {
			c.JSON(status, gin.H{"error": domainErr.Error()})
			return
		}
	}
	slog.Error("internal server error", "error", err.Error())
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
