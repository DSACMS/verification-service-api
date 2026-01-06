package circuitbreaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultOptions(t *testing.T) {
	expected := Options{
		FailureThreshold: 5,
		FailWindow:       10 * time.Second,
		OpenCoolDown:     30 * time.Second,
		HalfOpenLease:    5 * time.Second,
		FailOpen:         true,
		Prefix:           "cb:",
	}

	result := DefaultOptions()

	assert.Equalf(t, expected, result, "DefaultOptions() expected to equal %v; Got %v", expected, result)
}
