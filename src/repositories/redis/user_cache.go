package redis

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/repositories"
	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
)

type UserCache struct {
	db *rdb.Client
}

func NewUserCache(db *rdb.Client) repositories.UserCache {
	return &UserCache{
		db: db,
	}
}

func (c *UserCache) GetByID(ctx context.Context, id uuid.UUID) (*core.User, bool, error) {
	data, err := c.db.Get(ctx, id.String()).Bytes()
	if errors.Is(err, rdb.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var user *core.User
	err = json.Unmarshal([]byte(data), &user)
	if err != nil {
		return nil, false, err
	}

	return user, true, nil
}

func (c *UserCache) Set(ctx context.Context, user *core.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	err = c.db.Set(ctx, user.ID.String(), data, 0).Err()
	if err != nil {
		return err
	}
	return nil
}

func (c *UserCache) Delete(ctx context.Context, id uuid.UUID) error {
	return c.db.Del(ctx, id.String()).Err()
}
