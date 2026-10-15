package core

import (
	"time"

	"github.com/google/uuid"
)

type Passkey struct {
	ID         []byte
	UserID     uuid.UUID
	Data       []byte
	CreatedAt  time.Time
	LastUsedAt *time.Time
}
