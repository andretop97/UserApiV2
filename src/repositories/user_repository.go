package repositories

import (
	"context"
	"log/slog"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
)

type UserRepository struct {
	cache  UserCache
	source core.UserRepository
	logger *slog.Logger
}

func NewUserRepository(cache UserCache, source core.UserRepository, logger *slog.Logger) core.UserRepository {
	return &UserRepository{
		cache:  cache,
		source: source,
		logger: logger.With("component", "user_repository"),
	}
}

func (r *UserRepository) CreateUser(ctx context.Context, user *core.User) (*core.User, error) {
	return r.source.CreateUser(ctx, user)
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*core.User, error) {
	user, found, err := r.cache.GetByID(ctx, id)
	if err != nil {
		r.logger.WarnContext(ctx, "error getting user from cache", "user_id", id, "error", err)
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
		r.logger.WarnContext(ctx, "error setting user to cache", "user_id", id, "error", err)
	}

	return user, nil
}

func (r *UserRepository) GetUsersByName(ctx context.Context, name string) ([]*core.User, error) {
	return r.source.GetUsersByName(ctx, name)
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*core.User, error) {
	return r.source.GetUserByEmail(ctx, email)
}
func (r *UserRepository) GetAllUsers(ctx context.Context) ([]*core.User, error) {
	return r.source.GetAllUsers(ctx)
}
func (r *UserRepository) UpdateUser(ctx context.Context, user *core.User) (*core.User, error) {
	updatedUser, err := r.source.UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}
	err = r.cache.Delete(ctx, user.ID)
	if err != nil {
		r.logger.WarnContext(ctx, "cache invalidation failed", "user_id", user.ID, "error", err)
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
		r.logger.WarnContext(ctx, "cache invalidation failed", "user_id", id, "error", err)
	}
	return nil
}
