package core

import (
	"errors"
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

func (c MfaClaims) Validate() error {
	if c.UserID == uuid.Nil {
		return errors.New("missing user_id")
	}
	return nil
}

type SessionClaims struct {
	UserID uuid.UUID `json:"user_id"`
}

func (c SessionClaims) Validate() error {
	if c.UserID == uuid.Nil {
		return errors.New("missing user_id")
	}
	return nil
}
