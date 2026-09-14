package redis

import (
	"context"
	"time"

	rdb "github.com/redis/go-redis/v9"
)

type HealthCheck struct {
	client *rdb.Client
}

func NewHealthCheck(client *rdb.Client) *HealthCheck {
	return &HealthCheck{
		client: client,
	}
}

func (h *HealthCheck) HandShake() error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	err := h.client.Ping(ctx).Err()
	if err != nil {
		return err
	}
	return nil
}
