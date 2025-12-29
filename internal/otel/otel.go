package otel

import (
	"context"
	"errors"
	"fmt"

	"github.com/DSACMS/verification-service-api/internal/config"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var ServiceVersion string

func newConn() (*grpc.ClientConn, error) {
	conn, e := grpc.NewClient(
		config.AppConfig.Otel.OtlpExporter.Endpoint,
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

func newTraceProvider(ctx context.Context, res *resource.Resource, conn *grpc.ClientConn) (func(context.Context) error, error) {
	exp, e := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)

	if e != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", e)
	}

	bsp := sdktrace.NewBatchSpanProcessor(exp)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return provider.Shutdown, nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource, conn *grpc.ClientConn) (func(context.Context) error, error) {
	exp, e := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithGRPCConn(conn),
	)

	if e != nil {
		return nil, fmt.Errorf("failed to create meter exporter: %w", e)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(provider)

	return provider.Shutdown, nil
}

var Tracer trace.Tracer

func InitOtel(ctx context.Context) (func(context.Context) error, error) {
	res, err := newResource(ctx)
	if err != nil {
		return nil, err
	}

	conn, err := newConn()
	if err != nil {
		return nil, err
	}

	shutdownTracerProvider, err := newTraceProvider(ctx, res, conn)
	if err != nil {
		return nil, err
	}

	shutdownMeterProvider, err := newMeterProvider(ctx, res, conn)
	if err != nil {
		return nil, err
	}

	shutdownOtel := func(ctx context.Context) error {
		var err error

		traceErr := shutdownTracerProvider(ctx)
		meterErr := shutdownMeterProvider(ctx)

		if traceErr != nil {
			err = errors.Join(err, traceErr)
		}
		if meterErr != nil {
			err = errors.Join(err, meterErr)
		}

		return err
	}

	Tracer = otel.Tracer("verification-service-api")

	return shutdownOtel, nil
}

func StartSpan(ctx *fiber.Ctx, spanName string, opts ...trace.SpanStartOption) (trace.Span, func(opts ...trace.SpanEndOption)) {
	curCtx := ctx.UserContext()
	newCtx, span := Tracer.Start(curCtx, spanName, opts...)

	ctx.SetUserContext(newCtx)

	spanEndFn := func(options ...trace.SpanEndOption) {
		ctx.SetUserContext(curCtx)
		span.End(options...)
	}

	return span, spanEndFn
}
