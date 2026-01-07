package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultPassword     = ""
	defaultDB           = 0
	defaultDialTimeout  = 2 * time.Second
	defaultReadTimeout  = 2 * time.Second
	defaultWriteTimeout = 2 * time.Second
	defaultPoolTimeout  = 2 * time.Second

	defaultPoolSize     = 20
	defaultMinIdleConns = 2
)

type Config struct {
	// Typically "localhost:6379"
	Addr     string
	Password string
	DB       int
}

func NewClient(c Config) *redis.Client {
	opts := &redis.Options{
		Addr: c.Addr,
		// Password:     c.Password,
		Password: defaultPassword, // No password
		// DB:           c.DB,
		DB:           defaultDB,
		DialTimeout:  defaultDialTimeout,
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		PoolTimeout:  defaultPoolTimeout,
		PoolSize:     defaultPoolSize,
		MinIdleConns: defaultMinIdleConns,
	}

	return redis.NewClient(opts)
}

func Ping(ctx context.Context, rdb *redis.Client) error {
	return rdb.Ping(ctx).Err()
}
