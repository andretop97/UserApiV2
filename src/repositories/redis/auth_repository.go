package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
)

type AuthRepository struct {
	client *rdb.Client
}

func NewAuthRepository(client *rdb.Client) core.AuthRepository {
	return &AuthRepository{
		client: client,
	}
}

func mfaKey(id uuid.UUID) string {
	return fmt.Sprintf("auth:mfa:v1:%s", id.String())
}
func sessionKey(id uuid.UUID) string {
	return fmt.Sprintf("auth:session:v1:%s", id.String())
}
func ceremonyKey(id uuid.UUID, ceremony core.CeremonyType) string {
	return fmt.Sprintf("auth:webauthn:v1:%s:%s", ceremony, id.String())
}

func (r *AuthRepository) SetTemporaryToken(ctx context.Context, userID uuid.UUID, token string, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("save temporary token: ttl must be positive, got %s", ttl)
	}
	err := r.client.Set(ctx, mfaKey(userID), token, ttl).Err()
	return err
}

func (r *AuthRepository) GetTemporaryToken(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := r.client.Get(ctx, mfaKey(userID)).Result()
	if errors.Is(err, rdb.Nil) {
		return "", core.ErrTokenInvalid

	}
	if err != nil {
		return "", fmt.Errorf("get temporary token: %w", err)
	}
	return token, nil
}

func (r *AuthRepository) SetSessionToken(ctx context.Context, userID uuid.UUID, token string, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("save session token: ttl must be positive, got %s", ttl)
	}
	err := r.client.Set(ctx, sessionKey(userID), token, ttl).Err()
	return err
}

func (r *AuthRepository) GetSessionToken(ctx context.Context, userID uuid.UUID) (string, error) {
	token, err := r.client.Get(ctx, sessionKey(userID)).Result()
	if errors.Is(err, rdb.Nil) {
		return "", core.ErrTokenInvalid
	}
	if err != nil {
		return "", fmt.Errorf("get session token: %w", err)
	}
	return token, nil
}

func (r *AuthRepository) SaveCeremonySession(ctx context.Context, userID uuid.UUID, ceremony core.CeremonyType, session []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("save ceremony session: ttl must be positive, got %s", ttl)
	}
	err := r.client.Set(ctx, ceremonyKey(userID, ceremony), session, ttl).Err()
	return err
}

func (r *AuthRepository) ConsumeCeremonySession(ctx context.Context, userID uuid.UUID, ceremony core.CeremonyType) ([]byte, error) {
	session, err := r.client.GetDel(ctx, ceremonyKey(userID, ceremony)).Bytes()
	if errors.Is(err, rdb.Nil) {
		return nil, core.ErrCeremonyExpired
	}
	if err != nil {
		return nil, fmt.Errorf("consume ceremony session: %w", err)
	}
	return session, nil
}

func (r *AuthRepository) DeleteTemporaryToken(ctx context.Context, userID uuid.UUID) error {
	return r.client.Del(ctx, mfaKey(userID)).Err()

}
