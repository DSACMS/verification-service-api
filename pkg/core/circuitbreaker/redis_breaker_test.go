package circuitbreaker

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRedisBreaker(t *testing.T) {
	testRedisOpts := &redis.Options{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolTimeout:  2 * time.Second,
		PoolSize:     20,
		MinIdleConns: 2,
	}

	testBreakerOpts := Options{
		FailureThreshold: 5,
		FailWindow:       10 * time.Second,
		OpenCoolDown:     30 * time.Second,
		HalfOpenLease:    5 * time.Second,
		FailOpen:         true,
		Prefix:           "cb:",
	}

	rdb := redis.NewClient(testRedisOpts)

	result := NewRedisBreaker(rdb, "redisBreaker", testBreakerOpts)

	require.NotNil(t, result, "NewRedisBreaker should not return nil")

	assert.Same(t, rdb, result.rdb, "expected breaker to keep the passed-in redis client instance")

	assert.Equal(t, "redisBreaker", result.name)
	assert.Equal(t, testBreakerOpts, result.opts)

	require.NotNil(t, result.failScript, "expected failScript to be initialized")
}
