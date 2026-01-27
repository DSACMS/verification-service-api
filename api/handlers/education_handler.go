package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/DSACMS/verification-service-api/pkg/core"
	"github.com/DSACMS/verification-service-api/pkg/education"
	"github.com/gofiber/fiber/v2"
)

func TestEducationHandler(cfg *core.Config) fiber.Handler {
	const (
		contextTimeout time.Duration = 5 * time.Second
	)

	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), contextTimeout)
		defer cancel()

		reqBody := education.Request{
			AccountID:        cfg.NSC.AccountID,
			OrganizationName: "Lynette",
			DateOfBirth:      "1988-10-24",
			LastName:         "Oyola",
			FirstName:        "Lynette",
			Terms:            "y",
			EndClient:        "CMS",
		}

		result, err := education.TestEducationEndpoint(ctx, cfg, reqBody)
		if err != nil {
			log.Printf("education test failed: %v", err)

			return fiber.NewError(
				fiber.StatusBadGateway,
				fmt.Sprintf("education verification failed: %v", err),
			)
		}

		return c.Status(fiber.StatusOK).JSON(result)
	}

}
