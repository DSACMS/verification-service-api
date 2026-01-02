package logger

import (
	"io"
	"log/slog"

	"github.com/DSACMS/verification-service-api/internal/config"
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

func Setup(out io.Writer) {
	provider := log.NewLoggerProvider()
	otelHandler := otelslog.NewHandler(
		"verification-service-api",
		otelslog.WithLoggerProvider(provider),
	)

	var stdoutHandler slog.Handler
	if config.IsProd() {
		stdoutHandler = slog.NewTextHandler(out, &slog.HandlerOptions{})
	} else {
		stdoutHandler = slog.NewJSONHandler(out, &slog.HandlerOptions{})
	}

	Logger = slog.New(
		slogmulti.Fanout(
			stdoutHandler,
			otelHandler,
		),
	)

	cfg := slogfiber.Config{
		WithRequestID: true,
		WithSpanID:    true,
		WithTraceID:   true,
	}

	Middleware = slogfiber.NewWithConfig(Logger, cfg)
}
