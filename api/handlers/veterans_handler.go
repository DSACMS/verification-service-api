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

// type vaTokenMetaResponse struct {
// 	TokenType string `json:"token_type"`
// 	Scope     string `json:"scope"`
// 	ExpiresIn int    `json:"expires_in"`
// }

func VeteranAffairsHandler(cfg *core.VeteranAffairsConfig, vet veterans.VeteransService, logger *slog.Logger) fiber.Handler {
	const vaContextTimeout time.Duration = 5 * time.Second

	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(slog.String("handler", "VeteranAffairsHandler"))

	return func(c *fiber.Ctx) error {
		if cfg == nil {
			logger.Error("missing config\n\n")
			return fiber.NewError(fiber.StatusInternalServerError, "server misconfigured\n\n")
		}
		if vet == nil {
			logger.Error("missing veterans service\n\n")
			return fiber.NewError(fiber.StatusInternalServerError, "server misconfigured\n\n")
		}

		ctx, cancel := context.WithTimeout(c.Context(), vaContextTimeout)
		defer cancel()

		icn := c.Query("icn")
		if icn == "" {
			return fiber.NewError(fiber.StatusBadRequest, "missing icn")
		}

		scopes := []string{
			"launch",
			"veteran_status.read",
		}

		tok, err := vet.GetAccessToken(ctx, icn, scopes)
		if err != nil {
			logger.Error("failed to get VA access token\n\n", slog.Any("err", err))
			return fiber.NewError(fiber.StatusBadGateway, "failed to authenticate with VA\n\n")
		}
		if tok == nil {
			logger.Error("VA token response was nil\n\n")
			return fiber.NewError(fiber.StatusBadGateway, "failed to authenticate with VA\n\n")
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
