package circuitbreaker_test

import (
	. "github.com/DSACMS/verification-service-api/internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/DSACMS/verification-service-api/internal/circuitbreaker"
)

var _ = Describe("CircuitBreaker", func() {
	var cb *circuitbreaker.CircuitBreaker[any]
	var called bool

	BeforeEach(func() {
		called = false
		cb = circuitbreaker.NewCircuitBreaker(
			"test",
			func() (*any, error) {
				called = true
				return nil, nil
			},
		)
	})

	Context("when closed", func() {
		It("runs its wrapped function", func() {
			cb.Run(TODO())
			Expect(called).To(Equal(false))
		})
	})
})
