package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DSACMS/verification-service-api/api"
	"github.com/DSACMS/verification-service-api/pkg/core"

	"github.com/gofiber/fiber/v2"
)

var ErrRunFailed = errors.New("application failed to run")

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func run() error {
	logger := core.NewLogger(nil)

	err := core.LoadEnv()
	if err != nil {
		logger.Error(
			"Failed to load environment",
			"err",
			err,
		)
		return ErrRunFailed
	}

	cfg, err := core.NewConfigFromEnv()
	if err != nil {
		logger.Error(
			"Failed to get configuration",
			"err",
			err,
		)
		return ErrRunFailed
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	initLogger := core.NewLogger(&cfg)
	otel, err := core.NewOtelService(ctx, &cfg)
	if err != nil {
		initLogger.ErrorContext(
			ctx,
			"Otel error",
			"err",
			err,
		)
		return ErrRunFailed
	}
	defer otel.Shutdown(ctx, initLogger)

	logger := core.NewLoggerWithOtel(&cfg, otel)
	app, err := api.New(&api.Config{
		Core:   cfg,
		Logger: logger,
		Otel:   otel,
	})
	if err != nil {
		logger.ErrorContext(
			ctx,
			"Error building app",
			"err",
			err,
		)
		return ErrRunFailed
	}

	if err := runServer(ctx, app, ":8000"); err != nil {
		logger.ErrorContext(
			ctx,
			"Server error",
			"err",
			err,
		)
		return ErrRunFailed
	}

	return nil
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
