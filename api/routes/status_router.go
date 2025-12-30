package routes

import (
	"verification-service-api/api/handlers"

	"github.com/gofiber/fiber/v2"
)

func StatusRouter(app fiber.Router) {
	app.Get("/status", handlers.GetStatus())
}
