package otel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.Len(t, ended, 1, "expected exactly 1 ended span")
	assert.Equal(t, "hello", ended[0].Name(), "unexpected span name")
}
