package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusEndpoint(t *testing.T) {
	app := fiber.New()
	SetupRoutes(app)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)

	resp, err := app.Test(req)
	require.NoError(t, err, "failed to perform request")
	require.NotNil(t, resp)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
