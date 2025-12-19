package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DSACMS/verification-service-api/internal/middleware"
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
	if otelShutdown != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			otelShutdown(shutdownCtx)
		}()
	}

	app := buildApp()

	if err := runServer(ctx, app, ":8000"); err != nil {
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

	verifier, err := middleware.NewCognitoVerifier(middleware.CognitoConfig{
		Region:     os.Getenv("COGNITO_REGION"),
		UserPoolID: os.Getenv("COGNITO_USER_POOL_ID"),
		ClientID:   os.Getenv("COGNITO_APP_CLIENT_ID"),
	})
	if err != nil {
		log.Fatal(err)
	}

	app.Use(verifier.FiberMiddleware())

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

	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		return fmt.Errorf("error during shutdown: %w", err)
	}
	return nil
}
