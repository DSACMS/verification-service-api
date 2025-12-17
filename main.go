package main

import (
	"log"
	"syscall"
	"time"

	"github.com/DSACMS/verification-service-api/internal/router"
	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"context"
	"os"
	"os/signal"
)

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := otelShutdown(shutdownCtx)
		if err != nil {
			log.Printf("otel shutdown error: %v", err)
		}
	}()

	app := buildApp()

	err = runServer(ctx, app, ":8000")
	if err != nil {
		log.Printf("server error: %v", err)
	}
}

func buildApp() *fiber.App {
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "*",
		AllowMethods: "*",
	}))

	app.Use(otelfiber.Middleware())

	router.SetupRoutes(app)

	return app

}

func runServer(ctx context.Context, app *fiber.App, addr string) error {
	srvErr := make(chan error, 1)

	go func() {
		srvErr <- app.Listen(addr)
	}()

	select {
	case err := <-srvErr:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return app.ShutdownWithContext(shutdownCtx)
}
