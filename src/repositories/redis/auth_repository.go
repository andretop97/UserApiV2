package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/utils"
	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
)

type AuthRepository struct {
	client *rdb.Client
	ttl    time.Duration
}

func NewAuthRepository(client *rdb.Client, e *utils.AuthEnv) core.AuthRepository {
	return &AuthRepository{
		client: client,
		ttl:    e.LoginTokenTTL,
	}
}

func mfaKey(id uuid.UUID) string {
	return fmt.Sprintf("auth:mfa:v1:%s", id.String())
}
func sessionKey(id uuid.UUID) string {
	return fmt.Sprintf("auth:session:v1:%s", id.String())
}

func (r *AuthRepository) SetTemporaryToken(ctx context.Context, userID uuid.UUID, token string) error {
	err := r.client.Set(ctx, mfaKey(userID), token, r.ttl).Err()
	return err
}

func (r *AuthRepository) GetTemporaryToken(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := r.client.Get(ctx, mfaKey(userID)).Result()
	if err != nil {
		return "", err
	}
	return token, nil
}

func (r *AuthRepository) SetSessionToken(ctx context.Context, userID uuid.UUID, token string) error {
	err := r.client.Set(ctx, sessionKey(userID), token, r.ttl).Err()
	return err
}

func (r *AuthRepository) GetSessionToken(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := r.client.Get(ctx, sessionKey(userID)).Result()
	if err != nil {
		return "", err
	}
	return token, nil
}
