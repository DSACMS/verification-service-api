package logger

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup_NonProd_EmitsJSON(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")

	var buf bytes.Buffer
	Setup(&buf)

	require.NotNil(t, Logger)
	require.NotNil(t, Middleware)

	Logger.Info("hello")

	out := strings.TrimSpace(buf.String())
	require.NotEmpty(t, out)
	assert.True(t, strings.HasPrefix(out, "{"), out)
}

func TestSetup_Prod_EmitsText(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")

	var buf bytes.Buffer
	Setup(&buf)

	require.NotNil(t, Logger)
	require.NotNil(t, Middleware)

	Logger.Info("hello")

	out := strings.TrimSpace(buf.String())
	require.NotEmpty(t, out)
	assert.False(t, strings.HasPrefix(out, "{"), out)
}

func TestMiddleware_WiresIntoFiber(t *testing.T) {
	t.Setenv("ENVIRONMENT", "development")

	var buf bytes.Buffer
	Setup(&buf)

	app := fiber.New()
	app.Use(Middleware)
	app.Get("/ok", func(c *fiber.Ctx) error {
		return c.SendString("OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
