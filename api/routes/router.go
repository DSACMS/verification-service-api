package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/DSACMS/verification-service-api/api/handlers"
	"github.com/DSACMS/verification-service-api/api/middleware"
	"github.com/DSACMS/verification-service-api/pkg/circuitbreaker"
	"github.com/DSACMS/verification-service-api/pkg/core"
	"github.com/DSACMS/verification-service-api/pkg/education"
	"github.com/DSACMS/verification-service-api/pkg/veterans"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

const (
	timeout time.Duration = 10 * time.Second
)

func RegisterRoutes(app fiber.Router, cfg *core.Config, rdb *redis.Client, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Backend running!")
	})

	api := app.Group("/api")

	edu := education.New(&cfg.NSC, education.Options{
		Logger:     logger,
		HTTPClient: http.DefaultClient,
		Timeout:    timeout,
	})

	vetSvc, err := veterans.New(&cfg.VA, veterans.Options{
		Logger:     logger,
		HTTPClient: http.DefaultClient,
		Timeout:    timeout,
	})
	if err != nil {
		return fmt.Errorf("failed to init veterans service: %w", err)
	}

	withCB := middleware.WithCircuitBreaker(func(name string) *circuitbreaker.RedisBreaker {
		return circuitbreaker.NewRedisBreaker(
			rdb,
			name,
			circuitbreaker.DefaultOptions(),
			logger,
		)
	})
	withTimeout := middleware.WithRequestTimeout(timeout)

	api.Get("/edu", withCB(handlers.EducationHandler(cfg, edu, logger)))

	api.Get("/va", withCB(withTimeout(handlers.VeteranAffairsInfoHandler(logger))))
	api.Post("/va/disability-rating", withCB(withTimeout(handlers.VeteranAffairsDisabilityRatingHandler(vetSvc, logger))))

	return nil
}
