package redis_test

import (
	"testing"
	"time"

	"github.com/andretop97/UserApiV2/src/repositories/redis"
	rdb "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Testes do construtor não precisam de Redis: o cliente só conecta no primeiro comando.

func TestNewUserCache(t *testing.T) {
	client := rdb.NewClient(&rdb.Options{Addr: "localhost:1"})
	t.Cleanup(func() { client.Close() })

	t.Run("aceita TTL positivo", func(t *testing.T) {
		cache, err := redis.NewUserCache(client, time.Minute)

		require.NoError(t, err)
		assert.NotNil(t, cache)
	})

	// Para o go-redis, 0 e negativos significam "sem expiração" e -1 é KeepTTL:
	// qualquer um deles deixaria o cache sem expirar.
	invalid := map[string]time.Duration{
		"zero":           0,
		"KeepTTL (-1)":   rdb.KeepTTL,
		"negativo":       -time.Second,
		"menor negativo": -time.Nanosecond,
	}
	for name, ttl := range invalid {
		t.Run("rejeita TTL "+name, func(t *testing.T) {
			cache, err := redis.NewUserCache(client, ttl)

			assert.Error(t, err)
			assert.Nil(t, cache)
		})
	}
}
