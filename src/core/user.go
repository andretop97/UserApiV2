package core

import (
	"github.com/google/uuid"
)

type User struct {
	ID       uuid.UUID
	Name     string
	Email    string
	Password string
	Salt     string
}
