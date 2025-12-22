package router

import (
	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App) {
	app.Get("/status", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK) //200
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Backend running")
	})
}
