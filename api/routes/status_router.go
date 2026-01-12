package routes

import (
	"github.com/DSACMS/verification-service-api/api/handlers"
	"github.com/DSACMS/verification-service-api/api/middleware"
	"github.com/DSACMS/verification-service-api/pkg/circuitbreaker"
	"github.com/DSACMS/verification-service-api/pkg/core"
	"github.com/DSACMS/verification-service-api/pkg/redis"

	"github.com/gofiber/fiber/v2"
)

func StatusRouter(app fiber.Router, cfg core.Config) {

	rdb := redis.NewClient(redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	withBreaker := middleware.WithCircuitBreaker(func(name string) *circuitbreaker.RedisBreaker {
		return circuitbreaker.NewRedisBreaker(rdb, name, circuitbreaker.DefaultOptions())
	})

	app.Get("/status", withBreaker(handlers.GetRDBStatus(rdb)))
}
