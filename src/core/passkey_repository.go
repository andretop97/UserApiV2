package core

import (
	"context"

	"github.com/google/uuid"
)

type PasskeyRepository interface {
	Create(ctx context.Context, p *Passkey) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*Passkey, error)
	UpdateAfterLogin(ctx context.Context, id []byte, data []byte) error
}
