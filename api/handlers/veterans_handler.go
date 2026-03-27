package handlers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/DSACMS/verification-service-api/pkg/veterans"
	"github.com/gofiber/fiber/v2"
)

func VeteranAffairsInfoHandler(logger *slog.Logger) fiber.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	logger = logger.With(slog.String("handler", "VeteranAffairsInfoHandler"))

	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"availableEndpoint": "/api/va/disability-rating",
			"method":            fiber.MethodPost,
			"service":           "va",
			"status":            "ready",
		})
	}
}

func VeteranAffairsDisabilityRatingHandler(vet veterans.VeteransService, logger *slog.Logger) fiber.Handler {
	if vet == nil {
		panic("veterans service is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	logger = logger.With(slog.String("handler", "VeteranAffairsDisabilityRatingHandler"))

	return func(c *fiber.Ctx) error {
		var req veterans.DisabilityRatingRequest
		if err := c.BodyParser(&req); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
		}

		out, err := vet.GetDisabilityRating(c.UserContext(), c.Query("icn"), req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			if errors.Is(err, veterans.ErrICNRequired) {
				return fiber.NewError(fiber.StatusBadRequest, "'icn' is a required query parameter")
			}

			logger.Error("failed to get VA disability rating", slog.Any("error", err))
			return fiber.NewError(fiber.StatusBadGateway, "failed to get VA disability rating")
		}

		return c.Status(fiber.StatusOK).JSON(out)
	}
}
