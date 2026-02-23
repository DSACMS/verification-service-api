// pkg/handlers/veteran_affairs_handler.go
package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/DSACMS/verification-service-api/pkg/core"
	"github.com/DSACMS/verification-service-api/pkg/veterans"
	"github.com/gofiber/fiber/v2"
)

func VeteranAffairsHandler(cfg *core.VeteranAffairsConfig, vet veterans.VeteransService, logger *slog.Logger) fiber.Handler {
	const vaContextTimeout time.Duration = 5 * time.Second

	if logger == nil {
		logger = slog.Default()
	}

	logger = logger.With(slog.String("handler", "VeteranAffairsHandler"))

	return func(c *fiber.Ctx) error {
		if cfg == nil {
			logger.Error("missing config")
			return fiber.NewError(fiber.StatusInternalServerError, "server misconfigured")
		}
		if vet == nil {
			logger.Error("missing veterans service")
			return fiber.NewError(fiber.StatusInternalServerError, "server misconfigured")
		}

		ctx, cancel := context.WithTimeout(c.Context(), vaContextTimeout)
		defer cancel()

		tok, err := vet.GetAccessToken(ctx, veterans.DefaultTokenScopes)
		if err != nil {
			logger.Error("failed to get VA access token", slog.Any("err", err))
			return fiber.NewError(fiber.StatusBadGateway, "failed to authenticate with VA")
		}
		if tok == nil {
			logger.Error("VA token response was nil")
			return fiber.NewError(fiber.StatusBadGateway, "failed to authenticate with VA")
		}

		result := veterans.TokenResponse{
			TokenType: tok.TokenType,
			Scope:     tok.Scope,
			ExpiresIn: tok.ExpiresIn,
		}
		if result.TokenType == "" {
			result.TokenType = "Bearer"
		}

		return c.Status(fiber.StatusOK).JSON(result)
	}
}
