package resources

import (
	"errors"

	"github.com/DSACMS/verification-service-api/internal/config"
	"github.com/DSACMS/verification-service-api/internal/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

func newClient(ctx *fiber.Ctx) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: config.AppConfig.Redis.Addr,
	})

	err := errors.Join(
		redisotel.InstrumentTracing(rdb),
		redisotel.InstrumentMetrics(rdb),
	)
	if err != nil {
		logger.Logger.ErrorContext(
			ctx.Context(),
			"Failed to instrument redis client",
			"err",
			err,
		)
	}

	return rdb
}

func RedisClient(ctx *fiber.Ctx) *redis.Client {
	rdb, ok := ctx.Locals("rdb").(*redis.Client)
	if ok {
		return rdb
	}

	newRDB := newClient(ctx)
	ctx.Locals("rdb", newRDB)

	return newRDB
}
