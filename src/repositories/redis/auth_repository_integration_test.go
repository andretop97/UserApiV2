//go:build integration

package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/repositories/redis"
	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Usa o container e o testClient criados no TestMain de user_cache_integration_test.go.
//
//	go test -tags integration ./src/repositories/redis/...

const (
	mfaTTL     = 10 * time.Minute
	sessionTTL = 72 * time.Hour
)

func setupAuth(t *testing.T) core.AuthRepository {
	t.Helper()
	require.NoError(t, testClient.FlushDB(context.Background()).Err())
	return redis.NewAuthRepository(testClient)
}

func mfaKey(id uuid.UUID) string     { return "auth:mfa:v1:" + id.String() }
func sessionKey(id uuid.UUID) string { return "auth:session:v1:" + id.String() }

// assertWithin verifica um TTL do Redis com uma folga pequena para o tempo de execução do teste.
func assertWithin(t *testing.T, want, got time.Duration) {
	t.Helper()
	assert.InDelta(t, want.Seconds(), got.Seconds(), 5, "TTL esperado ~%s, veio %s", want, got)
}

func TestAuthRepositoryTemporaryToken(t *testing.T) {
	ctx := context.Background()

	t.Run("grava e lê o token MFA", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()

		require.NoError(t, repo.SetTemporaryToken(ctx, id, "mfa-token", mfaTTL))
		got, err := repo.GetTemporaryToken(ctx, id)

		require.NoError(t, err)
		assert.Equal(t, "mfa-token", got)
	})

	t.Run("usa a chave auth:mfa:v1:<id>", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()

		require.NoError(t, repo.SetTemporaryToken(ctx, id, "mfa-token", mfaTTL))

		got, err := testClient.Get(ctx, mfaKey(id)).Result()
		require.NoError(t, err)
		assert.Equal(t, "mfa-token", got)
	})

	t.Run("expira com o TTL informado", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()

		require.NoError(t, repo.SetTemporaryToken(ctx, id, "mfa-token", mfaTTL))

		ttl, err := testClient.TTL(ctx, mfaKey(id)).Result()
		require.NoError(t, err)
		assertWithin(t, mfaTTL, ttl)
	})

	t.Run("novo login substitui o token anterior", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()

		require.NoError(t, repo.SetTemporaryToken(ctx, id, "antigo", mfaTTL))
		require.NoError(t, repo.SetTemporaryToken(ctx, id, "novo", mfaTTL))
		got, err := repo.GetTemporaryToken(ctx, id)

		require.NoError(t, err)
		assert.Equal(t, "novo", got)
	})

	t.Run("token inexistente vira erro de domínio, sem vazar redis.Nil", func(t *testing.T) {
		repo := setupAuth(t)

		got, err := repo.GetTemporaryToken(ctx, uuid.New())

		require.Error(t, err)
		assert.Empty(t, got)
		assert.False(t, errors.Is(err, rdb.Nil), "o service não deveria depender de erros do go-redis")
		var domainErr *core.DomainError
		assert.True(t, errors.As(err, &domainErr), "esperado *core.DomainError, veio %T: %v", err, err)
	})
}

func TestAuthRepositorySessionToken(t *testing.T) {
	ctx := context.Background()

	t.Run("grava e lê o token de sessão", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()

		require.NoError(t, repo.SetSessionToken(ctx, id, "session-token", sessionTTL))
		got, err := repo.GetSessionToken(ctx, id)

		require.NoError(t, err)
		assert.Equal(t, "session-token", got)
	})

	t.Run("usa a chave auth:session:v1:<id>", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()

		require.NoError(t, repo.SetSessionToken(ctx, id, "session-token", sessionTTL))

		got, err := testClient.Get(ctx, sessionKey(id)).Result()
		require.NoError(t, err)
		assert.Equal(t, "session-token", got)
	})

	t.Run("expira com o TTL informado para a sessão", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()

		require.NoError(t, repo.SetSessionToken(ctx, id, "session-token", sessionTTL))

		ttl, err := testClient.TTL(ctx, sessionKey(id)).Result()
		require.NoError(t, err)
		assertWithin(t, sessionTTL, ttl)
	})

	t.Run("sessão inexistente vira erro de domínio, sem vazar redis.Nil", func(t *testing.T) {
		repo := setupAuth(t)

		got, err := repo.GetSessionToken(ctx, uuid.New())

		require.Error(t, err)
		assert.Empty(t, got)
		assert.False(t, errors.Is(err, rdb.Nil), "o service não deveria depender de erros do go-redis")
		var domainErr *core.DomainError
		assert.True(t, errors.As(err, &domainErr), "esperado *core.DomainError, veio %T: %v", err, err)
	})
}

// No go-redis, TTL 0 grava sem expiração e -1ns é KEEPTTL: o token valeria para sempre.
func TestAuthRepositoryRejectsNonPositiveTTL(t *testing.T) {
	ctx := context.Background()
	invalid := map[string]time.Duration{"zero": 0, "KeepTTL": rdb.KeepTTL, "negativo": -time.Minute}

	setters := map[string]struct {
		set func(repo core.AuthRepository, id uuid.UUID, ttl time.Duration) error
		key func(id uuid.UUID) string
	}{
		"token MFA": {
			set: func(repo core.AuthRepository, id uuid.UUID, ttl time.Duration) error {
				return repo.SetTemporaryToken(ctx, id, "mfa-token", ttl)
			},
			key: mfaKey,
		},
		"token de sessão": {
			set: func(repo core.AuthRepository, id uuid.UUID, ttl time.Duration) error {
				return repo.SetSessionToken(ctx, id, "session-token", ttl)
			},
			key: sessionKey,
		},
	}

	for kind, s := range setters {
		for name, ttl := range invalid {
			t.Run(kind+" com TTL "+name+" é recusado e nada é gravado", func(t *testing.T) {
				repo := setupAuth(t)
				id := uuid.New()

				err := s.set(repo, id, ttl)

				require.Error(t, err)
				var domainErr *core.DomainError
				assert.False(t, errors.As(err, &domainErr), "TTL inválido é erro do servidor, não do cliente")
				exists, err := testClient.Exists(ctx, s.key(id)).Result()
				require.NoError(t, err)
				assert.Zero(t, exists)
			})
		}
	}

	t.Run("TTL inválido não derruba o token MFA vigente", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()
		require.NoError(t, repo.SetTemporaryToken(ctx, id, "vigente", mfaTTL))

		require.Error(t, repo.SetTemporaryToken(ctx, id, "sem-ttl", 0))

		got, err := repo.GetTemporaryToken(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "vigente", got)
	})
}

func TestAuthRepositoryIsolation(t *testing.T) {
	ctx := context.Background()

	t.Run("token MFA e de sessão do mesmo usuário não colidem", func(t *testing.T) {
		repo := setupAuth(t)
		id := uuid.New()

		require.NoError(t, repo.SetTemporaryToken(ctx, id, "mfa-token", mfaTTL))
		require.NoError(t, repo.SetSessionToken(ctx, id, "session-token", sessionTTL))

		mfa, err := repo.GetTemporaryToken(ctx, id)
		require.NoError(t, err)
		session, err := repo.GetSessionToken(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "mfa-token", mfa)
		assert.Equal(t, "session-token", session)
	})

	t.Run("tokens de usuários diferentes não colidem", func(t *testing.T) {
		repo := setupAuth(t)
		a, b := uuid.New(), uuid.New()

		require.NoError(t, repo.SetTemporaryToken(ctx, a, "token-a", mfaTTL))
		require.NoError(t, repo.SetTemporaryToken(ctx, b, "token-b", mfaTTL))

		got, err := repo.GetTemporaryToken(ctx, a)
		require.NoError(t, err)
		assert.Equal(t, "token-a", got)
	})
}
