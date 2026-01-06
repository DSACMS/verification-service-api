package circuitbreaker

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type RedisBreaker struct {
	// Redis client used to read and update the circuit state.
	rdb *redis.Client
	// Name of the redis circuitbreaker that is used in combination with keys when constructing redis keys.
	name string
	// Defines the behaviour and timing characteristics of the breaker.
	opts Options
	//  Lua script to record failures atomically.
	failScript *redis.Script
}

var luaScript string = `
	local failsKey = KEYS[1]
	local openKey  = KEYS[2]
	local halfKey  = KEYS[3]

	local failWindowMs   = tonumber(ARGV[1])
	local threshold      = tonumber(ARGV[2])
	local openCooldownMs = tonumber(ARGV[3])

	-- increment failures
	local fails = redis.call("INCR", failsKey)

	-- ensure rolling window TTL exists
	local ttl = redis.call("PTTL", failsKey)
	if ttl < 0 then
	redis.call("PEXPIRE", failsKey, failWindowMs)
	end

	-- if threshold reached, open breaker
	if fails >= threshold then
	redis.call("SET", openKey, "1", "PX", openCooldownMs)
	redis.call("DEL", failsKey)
	redis.call("DEL", halfKey)
	return {fails, "opened"}
	end

	-- release probe lock after a failed attempt (so a later attempt can probe again)
	redis.call("DEL", halfKey)
	return {fails, "closed"}
	`

func NewRedisBreaker(rdb *redis.Client, name string, opts Options) *RedisBreaker {
	if opts.FailureThreshold <= 0 {
		opts = DefaultOptions()
	}

	b := &RedisBreaker{
		rdb:  rdb,
		name: name,
		opts: opts,
	}

	b.failScript = redis.NewScript(luaScript)

	return b
}

func (b *RedisBreaker) keys() (openKey, failsKey, halfKey string) {
	prefix := "cb:" + b.name + ":"
	return prefix + "open", prefix + "fails", prefix + "half"
}

// Allow returns nil if the call may proceed, or ErrCircuitOpen if it must be blocked.
func (b *RedisBreaker) Allow(ctx context.Context) error {
	openKey, _, _ := b.keys()

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
	_, failsKey, halfKey := b.keys()
	pipe := b.rdb.Pipeline()
	pipe.Del(ctx, failsKey)
	pipe.Del(ctx, halfKey)
	_, _ = pipe.Exec(ctx)
}

func (b *RedisBreaker) OnFailure(ctx context.Context) {
	openKey, failsKey, halfKey := b.keys()

	_, _ = b.failScript.Run(ctx, b.rdb,
		[]string{failsKey, openKey, halfKey},
		b.opts.FailWindow.Milliseconds(),
		b.opts.FailureThreshold,
		b.opts.OpenCoolDown.Milliseconds(),
	).Result()
}
