package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestStatusEndpoint(t *testing.T) {
	app := fiber.New()
	StatusRouter(app)

	req := httptest.NewRequest(http.MethodGet, "/status", http.NoBody)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("failed to perform request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status %d, got %d", fiber.StatusOK, resp.StatusCode)
	}
}
