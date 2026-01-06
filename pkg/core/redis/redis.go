package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
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
		Password: "", // No password
		// DB:           c.DB,
		DB:           0, // 0 is the default DB
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolTimeout:  2 * time.Second,
		PoolSize:     20,
		MinIdleConns: 2,
	}

	return redis.NewClient(opts)
}

func Ping(ctx context.Context, rdb *redis.Client) error {
	return rdb.Ping(ctx).Err()
}
