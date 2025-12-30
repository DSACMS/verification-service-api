package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type OtelService interface {
	SpanFromContext(c context.Context) trace.Span
	LoggerProvider() log.LoggerProvider
	Shutdown(c context.Context, logger *slog.Logger)
}

type otelService struct {
	meterProvider  *sdkmetric.MeterProvider
	tracerProvider *sdktrace.TracerProvider
	logProvider    *sdklog.LoggerProvider
	shutdown       func(context.Context) error
}

var _ OtelService = (*otelService)(nil)

func NewOtelService(ctx context.Context, cfg *Config) (OtelService, error) {
	res, err := newResource(ctx)
	if err != nil {
		return nil, err
	}

	conn, err := newConn(cfg)
	if err != nil {
		return nil, err
	}

	traceProvider, shutdownTraceProvider, err := newTraceProvider(ctx, res, conn)
	if err != nil {
		return nil, err
	}

	meterProvider, shutdownMeterProvider, err := newMeterProvider(ctx, res, conn)
	if err != nil {
		return nil, err
	}

	logProvider := sdklog.NewLoggerProvider(sdklog.WithResource(res))

	shutdown := func(shutdownCtx context.Context) error {
		return errors.Join(
			shutdownTraceProvider(shutdownCtx),
			shutdownMeterProvider(shutdownCtx),
		)
	}

	return &otelService{
		meterProvider:  meterProvider,
		tracerProvider: traceProvider,
		logProvider:    logProvider,
		shutdown:       shutdown,
	}, nil
}

func (s *otelService) LoggerProvider() log.LoggerProvider {
	return s.logProvider
}

func (*otelService) SpanFromContext(c context.Context) trace.Span {
	return trace.SpanFromContext(c)
}

func (s *otelService) Shutdown(c context.Context, logger *slog.Logger) {
	shutdownCtx, cancel := context.WithTimeout(c, 5*time.Second)
	defer cancel()

	err := s.shutdown(shutdownCtx)
	if err != nil {
		logger.ErrorContext(
			c,
			"Error shutting down otel",
			"err",
			err,
		)
	}
}

var ServiceVersion string

func newConn(cfg *Config) (*grpc.ClientConn, error) {
	conn, e := grpc.NewClient(
		cfg.Otel.OtlpExporter.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if e != nil {
		return nil, fmt.Errorf("failed to create GRPC connection: %w", e)
	}

	return conn, nil
}

func newResource(ctx context.Context) (*resource.Resource, error) {
	res, e := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithOS(),
		resource.WithProcess(),
		resource.WithContainer(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceVersion(ServiceVersion),
		),
	)

	if e == nil {
		return res, nil
	}
	return nil, fmt.Errorf("failed to create telemetry resource: %w", e)
}

func newTraceProvider(ctx context.Context, res *resource.Resource, conn *grpc.ClientConn) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	exp, e := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)

	if e != nil {
		return nil, nil, fmt.Errorf("failed to create trace exporter: %w", e)
	}

	bsp := sdktrace.NewBatchSpanProcessor(exp)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return provider, provider.Shutdown, nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource, conn *grpc.ClientConn) (*sdkmetric.MeterProvider, func(context.Context) error, error) {
	exp, e := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithGRPCConn(conn),
	)

	if e != nil {
		return nil, nil, fmt.Errorf("failed to create meter exporter: %w", e)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(provider)

	return provider, provider.Shutdown, nil
}
