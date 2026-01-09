package routes

import (
	"github.com/DSACMS/verification-service-api/api/handlers"
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

	app.Get("/status", handlers.GetRDBStatus(rdb))
}
