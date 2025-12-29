package logger

import (
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	slogfiber "github.com/samber/slog-fiber"
	slogmulti "github.com/samber/slog-multi"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/sdk/log"
)

var (
	Logger     *slog.Logger
	Middleware fiber.Handler
)

func init() {
	provider := log.NewLoggerProvider()
	otelHandler := otelslog.NewHandler(
		"verification-service-api",
		otelslog.WithLoggerProvider(provider),
	)
	stdoutHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{})

	Logger = slog.New(
		slogmulti.Fanout(
			stdoutHandler,
			otelHandler,
		),
	)

	config := slogfiber.Config{
		WithRequestID: true,
		WithSpanID:    true,
		WithTraceID:   true,
	}

	Middleware = slogfiber.NewWithConfig(Logger, config)
}
