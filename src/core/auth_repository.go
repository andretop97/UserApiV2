package core

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AuthRepository interface {
	SetTemporaryToken(ctx context.Context, userID uuid.UUID, token string, ttl time.Duration) error
	SetSessionToken(ctx context.Context, userID uuid.UUID, token string, ttl time.Duration) error
	GetTemporaryToken(ctx context.Context, userID uuid.UUID) (string, error)
	GetSessionToken(ctx context.Context, userID uuid.UUID) (string, error)
}
