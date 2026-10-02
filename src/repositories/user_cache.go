package repositories

import (
	"context"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
)

type UserCache interface {
	GetByID(ctx context.Context, id uuid.UUID) (*core.User, bool, error)
	Set(ctx context.Context, user *core.User) error
	Delete(ctx context.Context, id uuid.UUID) error
}
