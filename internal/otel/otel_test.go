package otel

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestOtel_EmitsSpan(t *testing.T) {
    ctx := context.Background()

    rec := tracetest.NewSpanRecorder()

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithResource(resource.Empty()),
        sdktrace.WithSpanProcessor(rec),
    )
    t.Cleanup(func() { _ = tp.Shutdown(ctx) })

    otel.SetTracerProvider(tp)

    tr := otel.Tracer("test")
    _, span := tr.Start(ctx, "hello")
    span.End()

    ended := rec.Ended()
    if len(ended) != 1 {
        t.Fatalf("expected 1 span, got %d", len(ended))
    }
    if ended[0].Name() != "hello" {
        t.Fatalf("expected span name 'hello', got %q", ended[0].Name())
    }
}

