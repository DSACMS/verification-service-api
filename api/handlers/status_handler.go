package handlers

import (
	"context"
	"time"

	redisLocal "github.com/DSACMS/verification-service-api/pkg/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// Build a handler that returns a 2** status when the service is
// running properly
func GetRDBStatus(rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 2*time.Second)
		defer cancel()

		err := redisLocal.Ping(ctx, rdb)
		if err != nil {
			return err
		}
		return c.SendStatus(fiber.StatusOK)
	}
}
