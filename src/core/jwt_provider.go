package core

import (
	"time"

	"github.com/google/uuid"
)

type JwtProvider[T any] interface {
	GenerateToken(payload T) (string, error)
	ValidateToken(token string) (T, error)
	TTL() time.Duration
}

type MfaClaims struct {
	UserID uuid.UUID `json:"user_id"`
}

type SessionClaims struct {
	UserID uuid.UUID `json:"user_id"`
}
