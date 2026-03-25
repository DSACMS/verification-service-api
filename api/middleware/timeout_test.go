package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func TestWithRequestTimeout_SetsUserContextDeadline(t *testing.T) {
	app := fiber.New()
	app.Get(
		"/ok",
		WithRequestTimeout(50*time.Millisecond)(func(c *fiber.Ctx) error {
			_, hasDeadline := c.UserContext().Deadline()
			require.True(t, hasDeadline)
			return c.SendStatus(fiber.StatusOK)
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/ok", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestWithRequestTimeout_ReturnsGatewayTimeout(t *testing.T) {
	app := fiber.New()
	app.Get(
		"/slow",
		WithRequestTimeout(5*time.Millisecond)(func(c *fiber.Ctx) error {
			<-c.UserContext().Done()
			return context.DeadlineExceeded
		}),
	)

	req := httptest.NewRequest(http.MethodGet, "/slow", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, fiber.StatusGatewayTimeout, resp.StatusCode)
}
