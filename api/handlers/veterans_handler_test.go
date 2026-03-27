package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DSACMS/verification-service-api/api/middleware"
	"github.com/DSACMS/verification-service-api/pkg/veterans"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
)

type fakeVeteransService struct {
	called bool
	fn     func(ctx context.Context, icn string, req veterans.DisabilityRatingRequest) (veterans.DisabilityRatingResponse, error)
}

func (f *fakeVeteransService) GetDisabilityRating(
	ctx context.Context,
	icn string,
	req veterans.DisabilityRatingRequest,
) (veterans.DisabilityRatingResponse, error) {
	f.called = true
	if f.fn != nil {
		return f.fn(ctx, icn, req)
	}
	return veterans.DisabilityRatingResponse{}, nil
}

func TestVeteranAffairsInfoHandler_ReturnsSafeMetadata(t *testing.T) {
	app := fiber.New()
	app.Get("/api/va", VeteranAffairsInfoHandler(nil))

	req := httptest.NewRequest(http.MethodGet, "/api/va", http.NoBody)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	require.Equal(t, "va", payload["service"])
	require.Equal(t, "/api/va/disability-rating", payload["availableEndpoint"])
	require.Equal(t, fiber.MethodPost, payload["method"])
	require.NotContains(t, payload, "access_token")
}

func TestVeteranAffairsDisabilityRatingHandler_PanicsWhenServiceIsNil(t *testing.T) {
	require.Panics(t, func() {
		_ = VeteranAffairsDisabilityRatingHandler(nil, nil)
	})
}

func TestVeteranAffairsDisabilityRatingHandler_RequiresICN(t *testing.T) {
	svc := &fakeVeteransService{
		fn: func(_ context.Context, icn string, _ veterans.DisabilityRatingRequest) (veterans.DisabilityRatingResponse, error) {
			if strings.TrimSpace(icn) == "" {
				return veterans.DisabilityRatingResponse{}, veterans.ErrICNRequired
			}
			return veterans.DisabilityRatingResponse{}, nil
		},
	}

	app := fiber.New()
	app.Post("/api/va/disability-rating", VeteranAffairsDisabilityRatingHandler(svc, nil))

	reqBody := `{"birth_date":"1990-01-01","zipcode":"12345"}`
	req := httptest.NewRequest(http.MethodPost, "/api/va/disability-rating", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	require.True(t, svc.called)
}

func TestVeteranAffairsDisabilityRatingHandler_InvalidBody(t *testing.T) {
	svc := &fakeVeteransService{}

	app := fiber.New()
	app.Post("/api/va/disability-rating", VeteranAffairsDisabilityRatingHandler(svc, nil))

	req := httptest.NewRequest(http.MethodPost, "/api/va/disability-rating?icn=100", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	require.False(t, svc.called)
}

func TestVeteranAffairsDisabilityRatingHandler_MapsServiceErrorToBadGateway(t *testing.T) {
	svc := &fakeVeteransService{
		fn: func(context.Context, string, veterans.DisabilityRatingRequest) (veterans.DisabilityRatingResponse, error) {
			return veterans.DisabilityRatingResponse{}, errors.New("upstream failed")
		},
	}

	app := fiber.New()
	app.Post("/api/va/disability-rating", VeteranAffairsDisabilityRatingHandler(svc, nil))

	reqBody := `{"birth_date":"1990-01-01","zipcode":"12345"}`
	req := httptest.NewRequest(http.MethodPost, "/api/va/disability-rating?icn=100", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, fiber.StatusBadGateway, resp.StatusCode)
}

func TestVeteranAffairsDisabilityRatingHandler_TimeoutMapsToGatewayTimeout(t *testing.T) {
	svc := &fakeVeteransService{
		fn: func(context.Context, string, veterans.DisabilityRatingRequest) (veterans.DisabilityRatingResponse, error) {
			return veterans.DisabilityRatingResponse{}, context.DeadlineExceeded
		},
	}

	app := fiber.New()
	handler := middleware.WithRequestTimeout(5 * time.Millisecond)(
		VeteranAffairsDisabilityRatingHandler(svc, nil),
	)
	app.Post("/api/va/disability-rating", handler)

	reqBody := `{"birth_date":"1990-01-01","zipcode":"12345"}`
	req := httptest.NewRequest(http.MethodPost, "/api/va/disability-rating?icn=100", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, fiber.StatusGatewayTimeout, resp.StatusCode)
}
