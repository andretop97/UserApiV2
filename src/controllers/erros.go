package controllers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func respondError(c *gin.Context, err error) {
	var domainErr *core.DomainError
	if errors.As(err, &domainErr) {
		c.JSON(domainErr.Status, gin.H{"error": gin.H{"code": domainErr.Code, "message": err.Error()}})
		return
	}
	slog.Error("internal server error", "error", err.Error())
	c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_SERVER_ERROR", "message": "internal server error"}})
}

func validationErrorMessage(err error) string {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return err.Error()
	}

	messages := make([]string, 0, len(ve))
	for _, fe := range ve {
		switch fe.Tag() {
		case "required":
			messages = append(messages, fmt.Sprintf("%s is required", fe.Field()))
		case "email":
			messages = append(messages, fmt.Sprintf("%s is invalid", fe.Field()))
		default:
			messages = append(messages, fmt.Sprintf("%s is invalid", fe.Field()))
		}
	}
	return strings.Join(messages, "; ")
}
