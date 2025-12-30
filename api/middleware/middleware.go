package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

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
	// if cfg.Region == "" || cfg.UserPoolID == "" || cfg.ClientID == "" {
	//		return nil, errors.New("Region, UserPoolID, and ClientID are required")
	// }

	// split to specify which is missing ^^^
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

	// if issuer == "" || jwksURL == "" {
	//		return nil, errors.New("issuer and jwksURL are required")
	// }

	// split to specify which is missing ^^^
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

		// 5 second limit to set up
		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
		defer cancel()

		// pull cached keys
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
		// save to c.locals to store termporary variables in the request's scope. They are only available to routes matching the request and go away when the request is handled
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
