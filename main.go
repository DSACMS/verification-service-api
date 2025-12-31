package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/DSACMS/verification-service-api/internal/logger"
	"github.com/DSACMS/verification-service-api/internal/middleware"
	"github.com/DSACMS/verification-service-api/internal/otel"
	"github.com/DSACMS/verification-service-api/internal/router"

	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownOtel, err := otel.InitOtel(ctx)
	if err != nil {
		logger.Logger.ErrorContext(ctx, "Otel error", "err", err)
		return err
	}

	if shutdownOtel != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if shutdownErr := shutdownOtel(shutdownCtx); shutdownErr != nil {
				logger.Logger.ErrorContext(shutdownCtx, "Error shutting down otel", "err", shutdownErr)
			}
		}()
	}

	app, err := buildApp(AppOptions{SkipAuth: false})
	if err != nil {
		logger.Logger.ErrorContext(ctx, "Error building app", "err", err)
		return err
	}

	if err := runServer(ctx, app, ":8000"); err != nil {
		logger.Logger.ErrorContext(ctx, "Server error", "err", err)
		return err
	}

	return nil
}

type AppOptions struct {
	SkipAuth bool
}

func buildApp(opts AppOptions) (*fiber.App, error) {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			span := trace.SpanFromContext(c.Context())
			span.RecordError(err)
			span.SetStatus(codes.Error, "Internal Service Error")

			logger.Logger.Error("Internal Service Error", "err", err)

			return c.Status(fiber.StatusInternalServerError).
				SendString(fiber.ErrInternalServerError.Message)
		},
	})

	app.Use(logger.Middleware)

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e any) {
			logger.Logger.ErrorContext(
				c.Context(),
				"panic!",
				"err", e,
				"stack", string(debug.Stack()),
			)
		},
	}))

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "*",
		AllowMethods: "*",
	}))

	app.Use(otelfiber.Middleware())

	if !opts.SkipAuth {
		verifier, err := middleware.NewCognitoVerifier(middleware.CognitoConfig{
			Region:     os.Getenv("COGNITO_REGION"),
			UserPoolID: os.Getenv("COGNITO_USER_POOL_ID"),
			ClientID:   os.Getenv("COGNITO_APP_CLIENT_ID"),
		})
		if err != nil {
			return nil, err
		}
		app.Use(verifier.FiberMiddleware())
	}

	// Routes
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

	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		return fmt.Errorf("error during shutdown: %w", err)
	}

	return nil
}
