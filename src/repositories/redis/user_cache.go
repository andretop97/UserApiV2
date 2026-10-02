package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/repositories"
	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
)

type cachedUser struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

func (u cachedUser) toUser() *core.User {
	return &core.User{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		DeletedAt: u.DeletedAt,
	}
}

func fromUser(user *core.User) *cachedUser {
	return &cachedUser{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		DeletedAt: user.DeletedAt,
	}
}

func userKey(id uuid.UUID) string {
	return fmt.Sprintf("cache:user:v1:%s", id.String())
}

func jitterDuration(duration time.Duration) time.Duration {
	jitter := time.Duration(float64(duration) * 0.1)

	jitter = duration + time.Duration(rand.Int63n(int64(jitter)))
	if jitter <= 0 {
		return duration
	}
	return jitter
}

type UserCache struct {
	db      *rdb.Client
	userTTL time.Duration
}

func NewUserCache(db *rdb.Client, userTTL time.Duration) (repositories.UserCache, error) {
	if userTTL <= 0 {
		return nil, fmt.Errorf("user cache: ttl must be positive, got %s", userTTL)
	}
	return &UserCache{
		db:      db,
		userTTL: userTTL,
	}, nil
}

func (c *UserCache) GetByID(ctx context.Context, id uuid.UUID) (*core.User, bool, error) {
	data, err := c.db.Get(ctx, userKey(id)).Bytes()
	if errors.Is(err, rdb.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("user cache get: %w", err)
	}

	var user cachedUser
	err = json.Unmarshal(data, &user)
	if err != nil {
		return nil, false, fmt.Errorf("user cache get: unmarshal error: %w", err)
	}

	if user.ID == uuid.Nil {
		return nil, false, fmt.Errorf("user cache get: user ID is nil in cached data")
	}
	return user.toUser(), true, nil
}

func (c *UserCache) Set(ctx context.Context, user *core.User) error {
	data, err := json.Marshal(fromUser(user))
	if err != nil {
		return fmt.Errorf("user cache set: marshal error: %w", err)
	}
	err = c.db.Set(ctx, userKey(user.ID), data, jitterDuration(c.userTTL)).Err()
	if err != nil {
		return fmt.Errorf("user cache set: %w", err)
	}
	return nil
}

func (c *UserCache) Delete(ctx context.Context, id uuid.UUID) error {
	err := c.db.Del(ctx, userKey(id)).Err()
	if errors.Is(err, rdb.Nil) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("user cache delete: %w", err)
	}

	return nil
}
