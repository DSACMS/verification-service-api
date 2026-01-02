package main

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/DSACMS/verification-service-api/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type routeTest struct {
	description  string
	route        string
	expectedCode int
	expectedBody string
}

func TestRoutes(t *testing.T) {
	logger.Setup(io.Discard)

	app, err := buildApp(AppOptions{SkipAuth: true})
	require.NoError(t, err, "buildApp error")

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
			require.NoErrorf(t, err, "http.NewRequest(%q) error", tt.route)

			resp, err := app.Test(req, -1)
			require.NoErrorf(t, err, "app.Test(%q) error", tt.route)
			require.NotNil(t, resp)
			defer resp.Body.Close()

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoErrorf(t, err, "io.ReadAll body error for %q", tt.route)

			body := strings.TrimSpace(string(bodyBytes))

			assert.Equalf(
				t,
				tt.expectedCode,
				resp.StatusCode,
				"unexpected status for %q. body=%q",
				tt.route,
				body,
			)

			if tt.expectedBody != "" {
				assert.Equalf(
					t,
					tt.expectedBody,
					body,
					"unexpected body for %q",
					tt.route,
				)
			}
		})
	}
}
