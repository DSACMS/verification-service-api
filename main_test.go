package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"verification-service-api/api"
	"verification-service-api/pkg/core"

	"github.com/gofiber/fiber/v2"
)

type routeTest struct {
	description  string
	route        string
	expectedBody string
	expectedCode int
}

func buildApp() (*fiber.App, error) {
	ctx := context.TODO()
	cfg, err := core.NewConfigFromEnv(
		core.WithEnvironment("test"),
		core.WithSkipAuth(),
		core.WithOtelDisable(),
	)
	if err != nil {
		return nil, err
	}

	otel, err := core.NewOtelService(ctx, &cfg)
	if err != nil {
		return nil, err
	}

	logger := core.NewLoggerWithOtel(&cfg, otel)

	return api.New((&api.Config{
		Config: cfg,
		Logger: logger,
		Otel:   otel,
	}))
}

func TestRoutes(t *testing.T) {
	app, err := buildApp()
	if err != nil {
		t.Fatalf("buildApp error: %v", err)
	}

	tests := []routeTest{
		{
			description:  "status route",
			route:        "/status",
			expectedCode: http.StatusOK,
			expectedBody: "OK",
		},
		{
			description:  "non existing route",
			route:        "/i-dont-exist",
			expectedCode: http.StatusNotFound,
			expectedBody: "Cannot GET /i-dont-exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, tt.route, http.NoBody)
			if err != nil {
				t.Fatalf("http.NewRequest error: %v", err)
			}

			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedCode {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf(
					"expected status %d, got %d. body=%q",
					tt.expectedCode,
					resp.StatusCode,
					strings.TrimSpace(string(body)),
				)
			}

			if tt.expectedBody != "" {
				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("io.ReadAll error: %v", err)
				}
				body := strings.TrimSpace(string(bodyBytes))

				if body != tt.expectedBody {
					t.Fatalf("expected body %q, got %q", tt.expectedBody, body)
				}
			}
		})
	}
}
