package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/yourusername/Verification-Service-API/router"
)

func main() {
	app := fiber.New()
	
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000/",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	router.SetupRoutes(app)

	// app.Get("/", func(c *fiber.Ctx) error {
	// 	return c.SendString("Hello World!")
	// })

	app.Listen(":8000")
}