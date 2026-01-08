package circuitbreaker

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type RedisBreaker struct {
	// Redis client used to read and update the circuit state.
	rdb *redis.Client
	// Name of the redis circuitbreaker that is used in combination with keys when constructing redis keys.
	name string
	// Defines the behaviour and timing characteristics of the breaker.
	opts Options
}

func NewRedisBreaker(rdb *redis.Client, name string, opts Options) *RedisBreaker {
	if opts.FailureThreshold <= 0 {
		opts = DefaultOptions()
	}

	b := &RedisBreaker{
		rdb:  rdb,
		name: name,
		opts: opts,
	}

	return b
}

func (b *RedisBreaker) keys() (openKey, failsKey string) {
	prefix := b.opts.Prefix + b.name + ":"
	return prefix + "open", prefix + "fails"
}

// Allow returns nil if the call may proceed, or ErrCircuitOpen if it must be blocked.
func (b *RedisBreaker) Allow(ctx context.Context) error {
	openKey, _ := b.keys()

	exists, err := b.rdb.Exists(ctx, openKey).Result()
	if err != nil {
		// If Redis is down:
		// fail-open (allow traffic) to keep service alive
		return nil
	}
	if exists == 1 {
		return ErrCircuitOpen
	}

	return nil
}

func (b *RedisBreaker) OnSuccess(ctx context.Context) {
	_, failsKey := b.keys()

	_ = b.rdb.Del(ctx, failsKey).Err()
}

func (b *RedisBreaker) OnFailure(ctx context.Context) {
	openKey, failsKey := b.keys()

	fails, err := b.rdb.Incr(ctx, failsKey).Result()
	if err != nil {
		return
	}

	ttl, err := b.rdb.PTTL(ctx, failsKey).Result()
	if err == nil && ttl < 0 {
		_ = b.rdb.PExpire(ctx, failsKey, b.opts.FailWindow).Err()
	}

	if int(fails) >= b.opts.FailureThreshold {
		// open breaker + reset counter
		_ = b.rdb.Set(ctx, openKey, "1", b.opts.OpenCoolDown).Err()
		_ = b.rdb.Del(ctx, failsKey).Err()
	}
}
