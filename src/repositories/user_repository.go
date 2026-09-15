package repositories

import (
	"context"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
)

type UserRepository struct {
	cache  core.UserRepository
	source core.UserRepository
}

func NewUserRepository(cache core.UserRepository, source core.UserRepository) core.UserRepository {
	return &UserRepository{
		cache:  cache,
		source: source,
	}
}

func (r *UserRepository) CreateUser(ctx context.Context, user *core.User) (*core.User, error) {
	createdUser, err := r.source.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	return createdUser, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	return nil, nil
}

func (r *UserRepository) GetUserByName(ctx context.Context, name string) ([]*core.User, error) {
	return nil, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	user, err := r.source.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	if user == nil {
		return nil, core.ErrUserNotFound
	}

	return user, nil
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
