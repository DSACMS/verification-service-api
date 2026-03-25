package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DSACMS/verification-service-api/pkg/core"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

func TestRegisterRoutes_ReturnsErrorWhenVeteransConfigInvalid(t *testing.T) {
	app := fiber.New()
	cfg := core.DefaultConfig()

	err := RegisterRoutes(app, &cfg, nil, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to init veterans service")
}

func TestRegisterRoutes_SucceedsAndRegistersHealthRoute(t *testing.T) {
	app := fiber.New()
	cfg := core.DefaultConfig()
	cfg.VA.ClientID = "client-id"
	cfg.VA.PrivateKeyPath = "test-key.pem"
	cfg.VA.TokenRecipientURL = "https://example.okta.com/oauth2/default/v1/token"
	cfg.VA.TokenURL = "https://sandbox-api.va.gov/oauth2/v1/token"

	err := RegisterRoutes(app, &cfg, nil, nil)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	resp, testErr := app.Test(req)
	require.NoError(t, testErr)
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.Equal(t, "Backend running!", string(body))
}
