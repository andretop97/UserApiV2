package redis_test

import (
	"testing"

	"github.com/andretop97/UserApiV2/src/repositories/redis"
	rdb "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestHealthCheckHandShakeUnavailable(t *testing.T) {
	client := rdb.NewClient(&rdb.Options{Addr: "localhost:1", MaxRetries: -1})
	t.Cleanup(func() { client.Close() })

	err := redis.NewHealthCheck(client).HandShake()

	assert.Error(t, err)
}
