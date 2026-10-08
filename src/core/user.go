package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID            uuid.UUID
	Name          string
	Email         string
	Password      string `json:"-"`
	PepperVersion int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

type UserUpdate struct {
	Name  *string
	Email *string
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func NormalizeName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" {
		return "", fmt.Errorf("%w: name cannot be empty", ErrBadRequest)
	}
	return n, nil
}
