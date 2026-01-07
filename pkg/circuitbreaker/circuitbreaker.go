package circuitbreaker

import (
	"context"
	"errors"
	"time"
)

var ErrOpen = errors.New("circuit breaker is open")

type Breaker interface {
	Allow(ctx context.Context) error
	OnSuccess(ctx context.Context)
	OnFailure(ctx context.Context)
}

type Options struct {
	// Number of failures before entering open state.
	FailureThreshold int
	// Time between failures to count as an outage.
	FailWindow time.Duration
	// How long to stay in open state before triggering half-open state.
	OpenCoolDown time.Duration
	// Time lease to allow only one pod instance at a time to test whether the circuit can be reopened.
	HalfOpenLease time.Duration
	// If Redis is unreachable (down, unavailable, timing out) and it's state is unknown, this determines the default behavior of the Allow method. What to do while breaker is blind.
	// TRUE: allows requests to proceed without circuit breaker participating
	// FALSE: blocks requests
	FailOpen bool
	// Key prefix to prevent name clashing.
	Prefix string
}

func DefaultOptions() Options {
	return Options{
		FailureThreshold: 5,
		FailWindow:       10 * time.Second,
		OpenCoolDown:     30 * time.Second,
		HalfOpenLease:    5 * time.Second,
		FailOpen:         true,
		Prefix:           "cb:",
	}
}
