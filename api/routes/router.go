package routes

import (
	"github.com/DSACMS/verification-service-api/api/handlers"
	"github.com/DSACMS/verification-service-api/pkg/core"
	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(app fiber.Router, cfg *core.Config) {
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Backend running!")
	})

	api := app.Group("/api")
	// RegisterRoutes(app, cfg)

	api.Get("/edu", handlers.TestEducationHandler(cfg))

}
