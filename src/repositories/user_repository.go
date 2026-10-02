package repositories

import (
	"context"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
)

type UserRepository struct {
	cache  UserCache
	source core.UserRepository
}

func NewUserRepository(cache UserCache, source core.UserRepository) core.UserRepository {
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
	user, found, err := r.cache.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if found {
		return user, nil
	}
	user, err = r.source.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	err = r.cache.Set(ctx, user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) GetUserByName(ctx context.Context, name string) ([]*core.User, error) {
	users, err := r.source.GetUserByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return users, nil
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
	users, err := r.source.GetAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	return users, nil
}
func (r *UserRepository) UpdateUser(ctx context.Context, user *core.User) (*core.User, error) {
	updatedUser, err := r.source.UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	err = r.cache.Delete(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}
func (r *UserRepository) DeleteUser(ctx context.Context, id uuid.UUID) error {
	err := r.source.DeleteUser(ctx, id)
	if err != nil {
		return err
	}
	err = r.cache.Delete(ctx, id)
	if err != nil {
		return err
	}
	return nil
}
