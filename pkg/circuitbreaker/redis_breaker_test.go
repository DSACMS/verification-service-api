package circuitbreaker

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	return redis.NewClient(&redis.Options{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolTimeout:  2 * time.Second,
		PoolSize:     20,
		MinIdleConns: 2,
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

	result := NewRedisBreaker(rdb, "redisBreaker", testBreakerOpts)

	require.NotNil(t, result, "NewRedisBreaker should not return nil")

	assert.Same(t, rdb, result.rdb, "Expected breaker to keep the passed-in redis client instance")

	assert.Equal(t, "redisBreaker", result.name)
	assert.Equal(t, testBreakerOpts, result.opts)

	require.NotNil(t, result.failScript, "Expected failScript to be initialized")
}

func TestNewRedisBreaker_keys(t *testing.T) {
	rdb := newTestRedisClient(t)
	testBreakerOpts := newTestBreakerOptions(t)

	breaker := NewRedisBreaker(rdb, "redisBreaker", testBreakerOpts)

	resultOpenKey, resultFailsKey, resultHalfKey := breaker.keys()

	expectedOpenKey := "cb:redisBreaker:open"
	expectedFailsKey := "cb:redisBreaker:fails"
	expectedHalfKey := "cb:redisBreaker:half"

	assert.Equalf(t, expectedOpenKey, resultOpenKey, "Got: %q; Expected: %q", expectedOpenKey, resultOpenKey)

	assert.Equalf(t, expectedFailsKey, resultFailsKey, "Got: %q; Expected: %q", expectedFailsKey, resultFailsKey)

	assert.Equalf(t, expectedHalfKey, resultHalfKey, "Got: %q; Expected: %q", expectedHalfKey, resultHalfKey)
}

func TestNewRedisBreaker_Allow(t *testing.T) {
	rdb := newTestRedisClient(t)
	testBreakerOpts := newTestBreakerOptions(t)

	breaker := NewRedisBreaker(rdb, "redisBreaker", testBreakerOpts)

	ctx := context.Background()

	err := breaker.Allow(ctx)

	require.NoErrorf(t, err, "The Allow method returned an error: %v", err)
}

// func TestNewRedisBreaker_OnSuccess(t *testing.T) {
// 	rdb := newTestRedisClient(t)m, 
// 	testBreakerOpts := newTestBreakerOptions(t)

// 	breaker := NewRedisBreaker(rdb, "redisBreaker", testBreakerOpts)

// }

 func TestRedisBreaker_OnFailure_TransitionsToOpen(t *testing.T) {
	rdb := newTestRedisClient(t)

	opts := newTestBreakerOptions(t)
	opts.FailureThreshold = 2

	ctx := context.Background()

	breaker := NewRedisBreaker(rdb, "redisBreaker", opts)

	openKey, failsKey, halfKey := breaker.keys()

	breaker.OnFailure(ctx)

	fails, err := rdb.Get(ctx, failsKey).Int64()
	require.NoError(t, err)
	require.Equal(t, int64(1), fails)

	exists, err := rdb.Exists(ctx, openKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists)

	exists, err = rdb.Exists(ctx, halfKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists)

	breaker.OnFailure(ctx)

	exists, err = rdb.Exists(ctx, openKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)

	exists, err = rdb.Exists(ctx, failsKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists)

	exists, err = rdb.Exists(ctx, halfKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists)

	err = breaker.Allow(ctx)
	require.ErrorIs(t, err, ErrCircuitOpen)
}

