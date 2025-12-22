package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DSACMS/verification-service-api/internal/config"
	"github.com/DSACMS/verification-service-api/internal/middleware"
	"github.com/DSACMS/verification-service-api/internal/otel"
	"github.com/DSACMS/verification-service-api/internal/router"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	log.Printf("Config: %+v\n", config.AppConfig)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownOtel, err := otel.InitOtel(ctx)
	if err != nil {
		log.Println(err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr := shutdownOtel(shutdownCtx)
		if shutdownErr != nil {
			log.Printf("Error during shutdown: %v", shutdownErr)
		}
	}()

	_, span := otel.Tracer.Start(ctx, "startup")
	span.AddEvent("Starting up")
	span.End()

	app, err := buildApp()
	if err != nil {
		log.Printf("Failed to build app: %v\n", err)
		return
	}

	if err := runServer(ctx, app, ":8000"); err != nil {
		log.Printf("server error: %v", err)
	}
}

func buildApp() (*fiber.App, error) {
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "*",
		AllowMethods: "*",
	}))

	app.Use(otelfiber.Middleware())

	verifier, err := middleware.NewCognitoVerifier(middleware.CognitoConfig{
		Region:     os.Getenv("COGNITO_REGION"),
		UserPoolID: os.Getenv("COGNITO_USER_POOL_ID"),
		ClientID:   os.Getenv("COGNITO_APP_CLIENT_ID"),
	})

	if err != nil {
		return nil, err
	}

	app.Use(verifier.FiberMiddleware())

	router.SetupRoutes(app)

	return app, nil
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

	// inline if since this err is only needed in the scope of this if statement.
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		return fmt.Errorf("error during shutdown: %w", err)
	}
	return nil
}
