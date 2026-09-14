package redis

import (
	"context"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
)

type UserRepository struct {
	db *rdb.Client
}

func NewUserRepository(db *rdb.Client) core.UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) CreateUser(ctx context.Context, user *core.User) (*core.User, error) {
	return nil, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	return nil, nil
}

func (r *UserRepository) GetUserByName(ctx context.Context, name string) (*core.User, error) {
	return nil, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	return nil, nil
}
func (r *UserRepository) GetAllUsers(ctx context.Context) ([]*core.User, error) {
	return nil, nil
}
func (r *UserRepository) UpdateUser(ctx context.Context, user *core.User) (*core.User, error) {
	return nil, nil
}
func (r *UserRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return nil
}
