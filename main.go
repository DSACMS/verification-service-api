package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DSACMS/verification-service-api/internal/router"
	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		log.Println(err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		otelShutdownErr := otelShutdown(shutdownCtx)
		if otelShutdownErr != nil {
			log.Printf("otel shutdown error: %v", otelShutdownErr)
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

	err := app.ShutdownWithTimeout(5 * time.Second)
	if err != nil {
		return fmt.Errorf("error during shutdown: %w", err)
	}
	return nil
}
