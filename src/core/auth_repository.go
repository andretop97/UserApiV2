package core

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CeremonyType string

const (
	CeremonyRegistration CeremonyType = "register"
	CeremonyLogin        CeremonyType = "login"
)

type AuthRepository interface {
	SetTemporaryToken(ctx context.Context, userID uuid.UUID, token string, ttl time.Duration) error
	SetSessionToken(ctx context.Context, userID uuid.UUID, token string, ttl time.Duration) error

	GetTemporaryToken(ctx context.Context, userID uuid.UUID) (string, error)
	GetSessionToken(ctx context.Context, userID uuid.UUID) (string, error)

	DeleteTemporaryToken(ctx context.Context, userID uuid.UUID) error

	SaveCeremonySession(ctx context.Context, userID uuid.UUID, ceremony CeremonyType, session []byte, ttl time.Duration) error
	ConsumeCeremonySession(ctx context.Context, userID uuid.UUID, ceremony CeremonyType) ([]byte, error)
}
