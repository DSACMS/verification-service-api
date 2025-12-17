package main

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

func setupOTelSDK(ctx context.Context) (func(context.Context) error, error) {
	var shutdownFuncs []func(context.Context) error
	var err error

	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	// propagator
	otel.SetTextMapPropagator(newPropagator())

	// pulls OTEL_SERVICE_NAME + OTEL_RESOURCE_ATTRIBUTES from env
	res, rErr := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
	if rErr != nil {
		handleErr(rErr)
		return shutdown, err
	}

	// traces
	tracerProvider, tpErr := newTracerProvider(res)
	if tpErr != nil {
		handleErr(tpErr)
		return shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// metrics
	meterProvider, mpErr := newMeterProvider(res)
	if mpErr != nil {
		handleErr(mpErr)
		return shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	// logs
	loggerProvider, lpErr := newLoggerProvider(res)
	if lpErr != nil {
		handleErr(lpErr)
		return shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
	global.SetLoggerProvider(loggerProvider)

	return shutdown, err
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTracerProvider(res *resource.Resource) (*trace.TracerProvider, error) {
	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	return trace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(exp, trace.WithBatchTimeout(time.Second)),
	), nil
}

func newMeterProvider(res *resource.Resource) (*metric.MeterProvider, error) {
	exp, err := stdoutmetric.New()
	if err != nil {
		return nil, err
	}

	return metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(exp, metric.WithInterval(120*time.Second))),
	), nil
}

func newLoggerProvider(res *resource.Resource) (*sdklog.LoggerProvider, error) {
	exp, err := stdoutlog.New()
	if err != nil {
		return nil, err
	}

	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
	), nil
}
