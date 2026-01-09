package core

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

	result := rec.Ended()
	expected := 1

	require.Len(t, result, expected, "expected %d ended span(s)", expected)

	expectedName := "hello"
	resultName := result[0].Name()

	assert.Equal(t, expectedName, resultName, "span name should match")
}
