package core

import (
	"context"

	"github.com/google/uuid"
)

type AuthRepository interface {
	SetTemporaryToken(ctx context.Context, userID uuid.UUID, token string) error
	SetSessionToken(ctx context.Context, userID uuid.UUID, token string) error
	GetTemporaryToken(ctx context.Context, userID uuid.UUID) (string, error)
	GetSessionToken(ctx context.Context, userID uuid.UUID) (string, error)
}
