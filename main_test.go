package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type routeTest struct {
	description  string
	route        string
	expectedCode int
	expectedBody string
}

func TestRoutes(t *testing.T) {
	app, err := buildApp(AppOptions{SkipAuth: true})
	if err != nil {
		t.Fatalf("buildApp error: %v", err)
	}

	tests := []routeTest{
		{
			description:  "index route",
			route:        "/",
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
			req, err := http.NewRequest(http.MethodGet, tt.route, nil)
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
