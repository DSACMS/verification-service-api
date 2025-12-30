package circuitbreaker

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/DSACMS/verification-service-api/internal/logger"
	"github.com/DSACMS/verification-service-api/internal/otel"
	"github.com/DSACMS/verification-service-api/internal/resources"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type State int

const (
	Closed State = iota
	HalfOpen
	Open
)

var ErrOpen = errors.New("circuit breaker open")

const (
	circuitBreakerOpenDuration   time.Duration = 2 * time.Hour
	circuitBreakerExpireDuration time.Duration = 4 * time.Hour
)

var stateName = map[State]string{
	Closed:   "CLOSED",
	HalfOpen: "HALF_OPEN",
	Open:     "OPEN",
}

func (s State) String() string {
	return stateName[s]
}

func saveOpenAt(ctx *fiber.Ctx, key string, openUntil time.Time) error {
	rdb := resources.RedisClient(ctx)
	openUntilStr := strconv.FormatInt(openUntil.Unix(), 10)
	_, err := rdb.Set(
		ctx.Context(),
		key,
		openUntilStr,
		circuitBreakerExpireDuration,
	).Result()
	if err != nil {
		return fmt.Errorf("failed to set value in cache: %w", err)
	}

	return nil
}

func saveClose(ctx *fiber.Ctx, key string) error {
	rdb := resources.RedisClient(ctx)
	_, err := rdb.Del(ctx.Context(), key).Result()
	if err != nil {
		return fmt.Errorf("failed to delete key in cache: %w", err)
	}

	return nil
}

func getOpenAt(ctx *fiber.Ctx, key string) (*time.Time, error) {
	rdb := resources.RedisClient(ctx)
	openUntilStr, getErr := rdb.Get(ctx.Context(), key).Result()
	if errors.Is(getErr, redis.Nil) {
		return nil, redis.Nil
	} else if getErr != nil {
		return nil, fmt.Errorf("failed to read value from cache: %w", getErr)
	}

	openUntilUnix, parseErr := strconv.ParseInt(openUntilStr, 10, 64)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse value as int64: %w", parseErr)
	}

	openUntil := time.Unix(openUntilUnix, 0)

	return &openUntil, nil
}

type CircuitBreaker[T any] struct {
	Fn func() (*T, error)
	ID string
}

func NewCircuitBreaker[T any](id string, fn func() (*T, error)) *CircuitBreaker[T] {
	return &CircuitBreaker[T]{
		ID: id,
		Fn: fn,
	}
}

func (c *CircuitBreaker[T]) Key() string {
	return "circuit-breaker:" + c.ID
}

func (c *CircuitBreaker[T]) SetOpen(ctx *fiber.Ctx) {
	span, endSpan := otel.StartSpan(
		ctx,
		"utils.circuit_breaker.SetOpen",
		trace.WithAttributes(
			attribute.String(
				"circuit_breaker.id",
				c.ID,
			),
		),
	)
	defer endSpan()

	err := saveOpenAt(
		ctx,
		c.Key(),
		time.Now().Add(circuitBreakerOpenDuration),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to open circuit breaker")

		logger.Logger.ErrorContext(
			ctx.Context(),
			"Failed to open circuit breaker",
			"circuit_breaker.id",
			c.ID,
			"err",
			err,
		)
	}
}

func (c *CircuitBreaker[T]) SetClosed(ctx *fiber.Ctx) {
	span, endSpan := otel.StartSpan(
		ctx,
		"utils.circuit_breaker.SetClosed",
		trace.WithAttributes(
			attribute.String(
				"circuit_breaker.id",
				c.ID,
			),
		),
	)
	defer endSpan()

	err := saveClose(
		ctx,
		c.Key(),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to open circuit breaker")

		logger.Logger.ErrorContext(
			ctx.Context(),
			"Failed to open circuit breaker",
			"circuit_breaker.id",
			c.ID,
			"err",
			err,
		)
	}
}

func (c *CircuitBreaker[T]) GetState(ctx *fiber.Ctx) State {
	_, endSpan := otel.StartSpan(
		ctx,
		"utils.circuit_breaker.GetOpen",
		trace.WithAttributes(
			attribute.String(
				"circuit_breaker.id",
				c.ID,
			),
		),
	)
	defer endSpan()

	openUntil, err := getOpenAt(ctx, c.Key())
	if err != nil && !errors.Is(err, redis.Nil) {
		logger.Logger.WarnContext(
			ctx.Context(),
			"Failed to get circuit breaker state",
			"circuit_breaker.id",
			c.ID,
			"err",
			err,
		)
		return Closed
	}

	if openUntil == nil {
		return Closed
	} else if time.Now().After(*openUntil) {
		// bump breaker timestamp to prevent additional runs
		c.SetOpen(ctx)
		return HalfOpen
	}

	return Open
}

func (c *CircuitBreaker[T]) Run(ctx *fiber.Ctx) (*T, error) {
	span, endSpan := otel.StartSpan(
		ctx,
		"utils.circuit_breaker.Run",
		trace.WithAttributes(
			attribute.String(
				"circuit_breaker.id",
				c.ID,
			),
		),
	)
	defer endSpan()

	trigger := func() {
		span.AddEvent("Run triggered circuit breaker")
		c.SetOpen(ctx)
	}

	state := c.GetState(ctx)

	if state == Open {
		span.AddEvent("Run blocked by open circuit breaker")
		return nil, ErrOpen
	}

	defer func() {
		if r := recover(); r != nil {
			trigger()
			// Rethrow so global error handler triggers
			panic(r)
		}
	}()

	result, err := c.Fn()
	if err != nil {
		trigger()
		return result, err
	}

	if state == HalfOpen {
		span.AddEvent("Circuit breaker has recovered")
		c.SetClosed(ctx)
	}

	return result, nil
}
