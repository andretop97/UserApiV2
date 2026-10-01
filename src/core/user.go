package core

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID            uuid.UUID
	Name          string
	Email         string
	Password      string
	PepperVersion int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}
