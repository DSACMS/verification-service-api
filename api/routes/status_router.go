package routes

import (
	"os"

	"github.com/DSACMS/verification-service-api/api/handlers"
	"github.com/DSACMS/verification-service-api/pkg/redis"

	"github.com/gofiber/fiber/v2"
)

func StatusRouter(app fiber.Router) {

	rdb := redis.NewClient(redis.Config{
		Addr: os.Getenv("REDIS_ADDR"), // or config module
	})

	app.Get("/status", handlers.GetRDBStatus(rdb))
}
