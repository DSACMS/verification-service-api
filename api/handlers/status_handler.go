package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// Build a handler that returns a 2** status when the service is
// running properly
func GetStatus() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	}
}
