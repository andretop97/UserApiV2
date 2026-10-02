//go:build integration

package redis_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/core"
	"github.com/andretop97/UserApiV2/src/repositories"
	"github.com/andretop97/UserApiV2/src/repositories/redis"
	"github.com/google/uuid"
	rdb "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

// Os testes de integração sobem um Redis descartável via testcontainers (requer Docker).
// O container é criado uma vez por pacote no TestMain e o banco é limpo antes de cada teste.
//
//	go test -tags integration ./src/repositories/redis/...

const redisImage = "redis:7.2"

var testClient *rdb.Client

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	container, err := tcredis.Run(ctx, redisImage)
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			fmt.Fprintln(os.Stderr, "encerrando container:", err)
		}
	}()
	if err != nil {
		fmt.Fprintln(os.Stderr, "subindo container:", err)
		return 1
	}

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connection string:", err)
		return 1
	}

	opts, err := rdb.ParseURL(uri)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse url:", err)
		return 1
	}
	testClient = rdb.NewClient(opts)
	defer testClient.Close()

	return m.Run()
}

const testTTL = time.Minute

func setup(t *testing.T) repositories.UserCache {
	t.Helper()
	return setupWithTTL(t, testTTL)
}

func setupWithTTL(t *testing.T, ttl time.Duration) repositories.UserCache {
	t.Helper()
	require.NoError(t, testClient.FlushDB(context.Background()).Err())
	cache, err := redis.NewUserCache(testClient, ttl)
	require.NoError(t, err)
	return cache
}

// unavailableCache aponta para uma porta sem Redis, para exercitar os caminhos de erro de conexão.
func unavailableCache(t *testing.T) repositories.UserCache {
	t.Helper()
	client := rdb.NewClient(&rdb.Options{Addr: "localhost:1", DialTimeout: 200 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { client.Close() })
	cache, err := redis.NewUserCache(client, testTTL)
	require.NoError(t, err)
	return cache
}

// cacheKey espelha o formato de chave usado pelo UserCache; atualize junto se o formato mudar.
func cacheKey(id uuid.UUID) string {
	return id.String()
}

func newUser() *core.User {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &core.User{
		ID:            uuid.New(),
		Name:          "João Silva",
		Email:         "joao@example.com",
		Password:      "hash-secreto",
		PepperVersion: 1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func TestUserCacheGetByID(t *testing.T) {
	ctx := context.Background()

	t.Run("chave inexistente é miss sem erro", func(t *testing.T) {
		cache := setup(t)

		got, found, err := cache.GetByID(ctx, uuid.New())

		require.NoError(t, err)
		assert.False(t, found)
		assert.Nil(t, got)
	})

	t.Run("payload que não é json retorna erro", func(t *testing.T) {
		cache := setup(t)
		id := uuid.New()
		require.NoError(t, testClient.Set(ctx, cacheKey(id), "não é json", 0).Err())

		got, found, err := cache.GetByID(ctx, id)

		assert.Error(t, err)
		assert.False(t, found)
		assert.Nil(t, got)
	})

	// JSON válido mas sem usuário (ID zerado) é tratado como miss, para o source repovoar a chave.
	emptyPayloads := map[string]string{
		"null":                     "null",
		"objeto vazio":             "{}",
		"campos desconhecidos":     `{"foo":"bar"}`,
		"id zerado explicitamente": `{"id":"00000000-0000-0000-0000-000000000000","name":"João"}`,
	}
	for name, payload := range emptyPayloads {
		t.Run("payload sem usuário é miss: "+name, func(t *testing.T) {
			cache := setup(t)
			id := uuid.New()
			require.NoError(t, testClient.Set(ctx, cacheKey(id), payload, 0).Err())

			got, found, err := cache.GetByID(ctx, id)

			require.NoError(t, err)
			assert.False(t, found)
			assert.Nil(t, got)
		})
	}

	t.Run("redis indisponível retorna erro", func(t *testing.T) {
		cache := unavailableCache(t)

		got, found, err := cache.GetByID(ctx, uuid.New())

		assert.Error(t, err)
		assert.False(t, found)
		assert.Nil(t, got)
	})
}

func TestUserCacheSet(t *testing.T) {
	ctx := context.Background()

	t.Run("grava e lê de volta os mesmos dados", func(t *testing.T) {
		cache := setup(t)
		user := newUser()

		require.NoError(t, cache.Set(ctx, user))
		got, found, err := cache.GetByID(ctx, user.ID)

		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, user.ID, got.ID)
		assert.Equal(t, user.Name, got.Name)
		assert.Equal(t, user.Email, got.Email)
		assert.True(t, user.CreatedAt.Equal(got.CreatedAt))
		assert.True(t, user.UpdatedAt.Equal(got.UpdatedAt))
		assert.Nil(t, got.DeletedAt)
	})

	t.Run("hash da senha não é gravado no cache", func(t *testing.T) {
		cache := setup(t)
		user := newUser()

		require.NoError(t, cache.Set(ctx, user))

		raw, err := testClient.Get(ctx, cacheKey(user.ID)).Result()
		require.NoError(t, err)
		assert.NotContains(t, raw, user.Password)

		got, _, err := cache.GetByID(ctx, user.ID)
		require.NoError(t, err)
		assert.Empty(t, got.Password)
	})

	t.Run("preserva DeletedAt preenchido", func(t *testing.T) {
		cache := setup(t)
		user := newUser()
		deletedAt := user.CreatedAt.Add(time.Hour)
		user.DeletedAt = &deletedAt

		require.NoError(t, cache.Set(ctx, user))
		got, _, err := cache.GetByID(ctx, user.ID)

		require.NoError(t, err)
		require.NotNil(t, got.DeletedAt)
		assert.True(t, deletedAt.Equal(*got.DeletedAt))
	})

	t.Run("sobrescreve valor existente", func(t *testing.T) {
		cache := setup(t)
		user := newUser()
		require.NoError(t, cache.Set(ctx, user))

		user.Name = "Nome Novo"
		require.NoError(t, cache.Set(ctx, user))
		got, _, err := cache.GetByID(ctx, user.ID)

		require.NoError(t, err)
		assert.Equal(t, "Nome Novo", got.Name)
	})

	t.Run("usuários diferentes não colidem", func(t *testing.T) {
		cache := setup(t)
		joao := newUser()
		maria := newUser()
		maria.Name = "Maria"

		require.NoError(t, cache.Set(ctx, joao))
		require.NoError(t, cache.Set(ctx, maria))

		got, _, err := cache.GetByID(ctx, joao.ID)
		require.NoError(t, err)
		assert.Equal(t, "João Silva", got.Name)
	})

	t.Run("grava com o TTL configurado", func(t *testing.T) {
		cache := setup(t)
		user := newUser()

		require.NoError(t, cache.Set(ctx, user))

		ttl, err := testClient.TTL(ctx, cacheKey(user.ID)).Result()
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Duration(0), "chave deve ter expiração")
		assert.LessOrEqual(t, ttl, testTTL)
	})

	t.Run("sobrescrever renova o TTL", func(t *testing.T) {
		cache := setup(t)
		user := newUser()
		require.NoError(t, cache.Set(ctx, user))
		require.NoError(t, testClient.Expire(ctx, cacheKey(user.ID), time.Second).Err())

		require.NoError(t, cache.Set(ctx, user))

		ttl, err := testClient.TTL(ctx, cacheKey(user.ID)).Result()
		require.NoError(t, err)
		assert.Greater(t, ttl, time.Second)
	})

	t.Run("chave expira depois do TTL", func(t *testing.T) {
		cache := setupWithTTL(t, 300*time.Millisecond)
		user := newUser()
		require.NoError(t, cache.Set(ctx, user))

		assert.Eventually(t, func() bool {
			_, found, err := cache.GetByID(ctx, user.ID)
			return err == nil && !found
		}, 3*time.Second, 50*time.Millisecond)
	})

	t.Run("redis indisponível retorna erro", func(t *testing.T) {
		cache := unavailableCache(t)

		err := cache.Set(ctx, newUser())

		assert.Error(t, err)
	})
}

func TestUserCacheDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("remove usuário do cache", func(t *testing.T) {
		cache := setup(t)
		user := newUser()
		require.NoError(t, cache.Set(ctx, user))

		require.NoError(t, cache.Delete(ctx, user.ID))
		got, found, err := cache.GetByID(ctx, user.ID)

		require.NoError(t, err)
		assert.False(t, found)
		assert.Nil(t, got)
	})

	t.Run("chave inexistente não é erro", func(t *testing.T) {
		cache := setup(t)

		err := cache.Delete(ctx, uuid.New())

		assert.NoError(t, err)
	})

	t.Run("remove apenas o usuário informado", func(t *testing.T) {
		cache := setup(t)
		joao := newUser()
		maria := newUser()
		require.NoError(t, cache.Set(ctx, joao))
		require.NoError(t, cache.Set(ctx, maria))

		require.NoError(t, cache.Delete(ctx, joao.ID))

		_, found, err := cache.GetByID(ctx, maria.ID)
		require.NoError(t, err)
		assert.True(t, found)
	})

	t.Run("redis indisponível retorna erro", func(t *testing.T) {
		cache := unavailableCache(t)

		err := cache.Delete(ctx, uuid.New())

		assert.Error(t, err)
	})
}

func TestHealthCheckHandShake(t *testing.T) {
	err := redis.NewHealthCheck(testClient).HandShake()

	assert.NoError(t, err)
}
