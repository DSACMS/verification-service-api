package circuitbreaker

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	redisClientAddr   = "localhost:6379"
	redisPassword     = ""
	redisDB           = 0
	redisDialTimeout  = 2 * time.Second
	redisReadTimeout  = 2 * time.Second
	redisWriteTimeout = 2 * time.Second
	redisPoolTimeout  = 2 * time.Second
	redisPoolSize     = 20
	redisMinIdleConns = 2
)

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	return redis.NewClient(&redis.Options{
		Addr:         redisClientAddr,
		Password:     redisPassword,
		DB:           redisDB,
		DialTimeout:  redisDialTimeout,
		ReadTimeout:  redisReadTimeout,
		WriteTimeout: redisWriteTimeout,
		PoolTimeout:  redisPoolTimeout,
		PoolSize:     redisPoolSize,
		MinIdleConns: redisMinIdleConns,
	})
}

func newTestBreakerOptions(t *testing.T) Options {
	t.Helper()

	return Options{
		FailureThreshold: 5,
		FailWindow:       10 * time.Second,
		OpenCoolDown:     30 * time.Second,
		HalfOpenLease:    5 * time.Second,
		FailOpen:         true,
		Prefix:           "cb:",
	}
}

func TestNewRedisBreaker(t *testing.T) {
	rdb := newTestRedisClient(t)
	testBreakerOpts := newTestBreakerOptions(t)

	result := NewRedisBreaker(rdb, "redisBreaker"+t.Name(), testBreakerOpts)

	require.NotNil(t, result, "NewRedisBreaker should not return nil")

	assert.Same(t, rdb, result.rdb, "Expected breaker to keep the passed-in redis client instance")

	assert.Equal(t, "redisBreaker", result.name)
	assert.Equal(t, testBreakerOpts, result.opts)
}

func TestNewRedisBreaker_keys(t *testing.T) {
	rdb := newTestRedisClient(t)
	testBreakerOpts := newTestBreakerOptions(t)

	breaker := NewRedisBreaker(rdb, "redisBreaker"+t.Name(), testBreakerOpts)

	resultOpenKey, resultFailsKey := breaker.keys()

	expectedOpenKey := "cb:redisBreaker:open"
	expectedFailsKey := "cb:redisBreaker:fails"

	assert.Equalf(t, expectedOpenKey, resultOpenKey, "Got: %q; Expected: %q", expectedOpenKey, resultOpenKey)

	assert.Equalf(t, expectedFailsKey, resultFailsKey, "Got: %q; Expected: %q", expectedFailsKey, resultFailsKey)
}

func TestNewRedisBreaker_Allow(t *testing.T) {
	rdb := newTestRedisClient(t)
	testBreakerOpts := newTestBreakerOptions(t)

	breaker := NewRedisBreaker(rdb, "redisBreaker"+t.Name(), testBreakerOpts)

	ctx := context.Background()

	err := breaker.Allow(ctx)

	require.NoErrorf(t, err, "The Allow method returned an error: %v", err)
}

func TestRedisBreaker_OnFailure_TransitionsToOpen(t *testing.T) {
	rdb := newTestRedisClient(t)

	opts := newTestBreakerOptions(t)
	opts.FailureThreshold = 2

	ctx := context.Background()

	breaker := NewRedisBreaker(rdb, "redisBreaker"+t.Name(), opts)

	openKey, failsKey := breaker.keys()

	breaker.OnFailure(ctx)

	fails, err := rdb.Get(ctx, failsKey).Int64()
	require.NoError(t, err)
	require.Equal(t, int64(1), fails)

	exists, err := rdb.Exists(ctx, openKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists)

	breaker.OnFailure(ctx)

	exists, err = rdb.Exists(ctx, openKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)

	exists, err = rdb.Exists(ctx, failsKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists)

	err = breaker.Allow(ctx)
	require.ErrorIs(t, err, ErrCircuitOpen)
}
