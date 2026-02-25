package handlers

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/DSACMS/verification-service-api/pkg/core"
	"github.com/DSACMS/verification-service-api/pkg/veterans"
	"github.com/gofiber/fiber/v2"
)

func VeteranAffairsHandler(cfg *core.VeteranAffairsConfig, vet veterans.VeteransService, logger *slog.Logger) fiber.Handler {
	const vaContextTimeout time.Duration = 10 * time.Second

	if logger == nil {
		logger = slog.Default()
	}

	logger = logger.With(slog.String("handler", "VeteranAffairsHandler"))

	return func(c *fiber.Ctx) error {
		if cfg == nil || vet == nil {
			logger.Error("server misconfigured")
			return fiber.NewError(fiber.StatusInternalServerError, "server misconfigured")
		}

		icn := strings.TrimSpace(c.Query("icn"))
		if icn == "" {
			return fiber.NewError(fiber.StatusBadRequest, "missing required query parameter: icn")
		}

		ctx, cancel := context.WithTimeout(c.Context(), vaContextTimeout)
		defer cancel()

		// POST /api/va/disability-rating
		if c.Method() == fiber.MethodPost && strings.HasSuffix(c.Path(), "/va/disability-rating") {
			var req veterans.DisabilityRatingRequest
			if err := c.BodyParser(&req); err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
			}

			out, err := vet.GetDisabilityRating(ctx, icn, req)
			if err != nil {
				logger.Error("failed to get VA disability rating", slog.Any("err", err))
				return fiber.NewError(fiber.StatusBadGateway, err.Error())
			}
			return c.Status(fiber.StatusOK).JSON(out)
		}

		// GET /api/va (token check)
		tok, err := vet.GetAccessToken(ctx, icn, veterans.DefaultTokenScopes)
		if err != nil {
			logger.Error("failed to get VA access token", slog.Any("err", err))
			return fiber.NewError(fiber.StatusBadGateway, "failed to authenticate with VA")
		}
		if tok == nil {
			logger.Error("VA token response was nil")
			return fiber.NewError(fiber.StatusBadGateway, "failed to authenticate with VA")
		}

		return c.Status(fiber.StatusOK).JSON(veterans.TokenResponse{
			TokenType: tok.TokenType,
			Scope:     tok.Scope,
			ExpiresIn: tok.ExpiresIn,
		})
	}
}
