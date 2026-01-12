package circuitbreaker

import (
	"context"
	"log"
	"strconv"
	"time"

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

var _ Breaker = (*RedisBreaker)(nil)

func NewRedisBreaker(rdb *redis.Client, name string, opts Options) *RedisBreaker {
	if opts.FailureThreshold <= 0 {
		opts = DefaultOptions()
	}
	if opts.Prefix == "" {
		opts.Prefix = "cb:"
	}
	return &RedisBreaker{rdb: rdb, name: name, opts: opts}
}

func (b *RedisBreaker) keys() (stateKey, failsKey string) {
	base := b.opts.Prefix + b.name
	return base, base + ":fails"
}

func (b *RedisBreaker) Allow(ctx context.Context) error {
	stateKey, _ := b.keys()

	val, err := b.rdb.Get(ctx, stateKey).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		if b.opts.FailOpen {
			log.Printf("Redis GET failed; defaulting to allow (assume closed). name=%q key=%q err=%v", b.name, stateKey, err)
			return nil
		}
		return ErrCircuitOpen
	}

	timeToHalfOpenMs, convErr := strconv.ParseInt(val, 10, 64)
	if convErr != nil {
		if b.opts.FailOpen {
			log.Printf("Invalid redis value; defaulting to allow (assume closed). name=%q key=%q err=%v", b.name, stateKey, err)
			return nil
		}
		return ErrCircuitOpen
	}

	nowMs := time.Now().UnixMilli()

	if nowMs >= timeToHalfOpenMs {
		return nil
	}

	return ErrCircuitOpen
}

func (b *RedisBreaker) OnSuccess(ctx context.Context) {
	stateKey, failsKey := b.keys()
	_ = b.rdb.Del(ctx, stateKey, failsKey).Err()
}

func (b *RedisBreaker) OnFailure(ctx context.Context) {
	stateKey, failsKey := b.keys()

	fails, err := b.rdb.Incr(ctx, failsKey).Result()
	if err != nil {
		return
	}

	ttl, err := b.rdb.PTTL(ctx, failsKey).Result()
	if err == nil && ttl < 0 {
		_ = b.rdb.PExpire(ctx, failsKey, b.opts.FailWindow).Err()
	}

	if int(fails) >= b.opts.FailureThreshold {
		timeToHalfOpenMs := time.Now().Add(b.opts.OpenCoolDown).UnixMilli()

		stateTTL := b.opts.OpenCoolDown + b.opts.HalfOpenLease
		if stateTTL <= 0 {
			stateTTL = b.opts.OpenCoolDown
		}

		_ = b.rdb.Set(ctx, stateKey, strconv.FormatInt(timeToHalfOpenMs, 10), stateTTL).Err()

		_ = b.rdb.Del(ctx, failsKey).Err()
	}
}
