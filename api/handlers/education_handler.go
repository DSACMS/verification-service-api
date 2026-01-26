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
			AccountID:        "xxxxxxxx",
			OrganizationName: "Account Name",
			CaseReferenceID:  "xxxxxxx",
			ContactEmail:     "xxxxx@xxx.org",
			DateOfBirth:      "YYYYMMDD",
			LastName:         "Doe",
			FirstName:        "John",
			SSN:              "xxxxxxxxx",
			IdentityDetails: []education.IdentityDetails{
				{
					ElementName:  "degreeDetails/degreeTitle",
					ElementValue: "MASTER OF ENVIRONMENTAL ENGINEERING",
				},
				{
					ElementName:  "degreeDetails/majorCoursesOfStudy/course",
					ElementValue: "MATH",
				},
			},
		}

		result, err := education.TestEducationEndpoint(ctx, cfg, reqBody)
		if err != nil {
			log.Printf("education test failed: %v", err)

			// return a safe message, but keep the upstream detail
			return fiber.NewError(
				fiber.StatusBadGateway,
				fmt.Sprintf("education verification failed: %v", err),
			)
		}

		return c.Status(fiber.StatusOK).JSON(result)
	}

}
