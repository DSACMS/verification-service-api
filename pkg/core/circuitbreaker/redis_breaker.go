package circuitbreaker

import (
	"github.com/redis/go-redis/v9"
)

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

// func (b *RedisBreaker) keys() (openKey, failsKey, halfKey)
