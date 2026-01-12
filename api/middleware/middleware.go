package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/DSACMS/verification-service-api/pkg/circuitbreaker"
	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type CognitoConfig struct {
	Region     string
	UserPoolID string
	ClientID   string
}

type CognitoVerifier struct {
	issuer  string
	jwksURL string
	cache   *jwk.Cache
	client  *http.Client
	cfg     CognitoConfig
}

func NewCognitoVerifier(cfg CognitoConfig) (*CognitoVerifier, error) {
	if cfg.Region == "" {
		return nil, errors.New("Region is required")
	}

	if cfg.UserPoolID == "" {
		return nil, errors.New("UserPoolID is required")
	}

	if cfg.ClientID == "" {
		return nil, errors.New("ClientID is required")
	}

	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", cfg.Region, cfg.UserPoolID)
	jwksURL := issuer + "/.well-known/jwks.json"

	cache := jwk.NewCache(context.Background())
	// register the JWKS URL with a refresh window
	cache.Register(jwksURL)

	return &CognitoVerifier{
		issuer:  issuer,
		jwksURL: jwksURL,
		cache:   cache,
		client:  &http.Client{Timeout: 5 * time.Second},
		cfg:     cfg,
	}, nil
}

// copy for testing
func NewCognitoVerifierWithURLs(cfg CognitoConfig, issuer, jwksURL string) (*CognitoVerifier, error) {
	if cfg.ClientID == "" {
		return nil, errors.New("ClientID is required")
	}

	if issuer == "" {
		return nil, errors.New("issuer is required")
	}

	if jwksURL == "" {
		return nil, errors.New("jwksURL is required")
	}

	cache := jwk.NewCache(context.Background())
	cache.Register(jwksURL)

	return &CognitoVerifier{
		issuer:  issuer,
		jwksURL: jwksURL,
		cache:   cache,
		client:  &http.Client{Timeout: 5 * time.Second},
		cfg:     cfg,
	}, nil
}

func (v *CognitoVerifier) FiberMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := c.Get("x-amzn-oidc-accesstoken")
		if raw == "" {
			return fiber.ErrUnauthorized
		}

		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
		defer cancel()

		keyset, err := v.cache.Get(ctx, v.jwksURL)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "unable to load jwks")
		}

		tok, err := jwt.Parse(
			[]byte(raw),
			jwt.WithKeySet(keyset),
			jwt.WithValidate(true),

			jwt.WithIssuer(v.issuer),

			jwt.WithClaimValue("token_use", "access"),
		)
		if err != nil {
			return fiber.ErrUnauthorized
		}

		// access tokens commonly carry client id in "client_id"
		// wrote return values + call in if statement since these are one-offs.
		// when written like this, the return values aren't accessible outside the scope of this if statement
		if cid, ok := tok.Get("client_id"); !ok || cid != v.cfg.ClientID {
			return fiber.ErrUnauthorized
		}

		// put useful info on context
		// save to c.locals to store temporary variables in the request's scope. They are only available to routes matching the request and go away when the request is handled
		if sub, ok := tok.Get("sub"); ok {
			c.Locals("sub", sub)
		}
		if username, ok := tok.Get("username"); ok {
			c.Locals("username", username)
		}
		if scope, ok := tok.Get("scope"); ok {
			c.Locals("scope", scope)
		}
		if groups, ok := tok.Get("cognito:groups"); ok {
			c.Locals("groups", groups)
		}

		return c.Next()
	}
}

func WithCircuitBreaker(newBreaker func(name string) *circuitbreaker.RedisBreaker) func(fiber.Handler) fiber.Handler {
	var mu sync.RWMutex
	breakers := make(map[string]*circuitbreaker.RedisBreaker)

	getBreaker := func(name string) *circuitbreaker.RedisBreaker {
		mu.RLock()
		b := breakers[name]
		mu.RUnlock()
		if b != nil {
			return b
		}

		mu.Lock()
		defer mu.Unlock()
		if b = breakers[name]; b != nil {
			return b
		}

		b = newBreaker(name)
		breakers[name] = b
		return b
	}

	return func(next fiber.Handler) fiber.Handler {
		return func(c *fiber.Ctx) error {
			name := breakerName(c)
			breaker := getBreaker(name)

			err := breaker.Allow(c.Context())
			if err != nil {
				if errors.Is(err, circuitbreaker.ErrCircuitOpen) {
					return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
						"error": "service temporarily unavailable",
						"code":  "CIRCUIT_OPEN",
					})
				}

				return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
					"error": "service temporarilt unavailable",
					"code":  "BREAKER_ERROR",
				})
			}

			return next(c)
		}
	}
}

func breakerName(c *fiber.Ctx) string {
	var path string
	r := c.Route()
	if r != nil && r.Path != "" {
		path = r.Path
	} else {
		path = c.Path()
	}

	return c.Method() + " " + path

}
