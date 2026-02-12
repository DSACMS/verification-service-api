package veterans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/DSACMS/verification-service-api/pkg/core"
)

const (
	grantType           = "client_credentials"
	clientAssertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
	contentTypeForm     = "application/x-www-form-urlencoded"
	acceptJSON          = "application/json"

	tokenReuseSkewSeconds = 30 // refresh a bit early
)

type VeteransService interface {
	Submit(ctx context.Context, req DisabilityRatingRequest) (DisabilityRatingResponse_200, error)
	GetAccessToken(ctx context.Context, icn string, scopes []string) (*AccessToken, error)
}

type HTTPTransport interface {
	Do(req *http.Request) (*http.Response, error)
}

type Options struct {
	// Override for testing / custom transport (http.Client implements this).
	HTTPClient HTTPTransport
	// Structured logger
	Logger *slog.Logger
	// Per-call timeout when caller context has no deadline.
	Timeout time.Duration
}

type service struct {
	cfg     *core.VeteranAffairsConfig
	client  HTTPTransport
	logger  *slog.Logger
	timeout time.Duration

	mu          sync.Mutex
	cachedToken *AccessToken
	tokenExpAt  time.Time
}

type AccessToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
}

// New creates the veterans service.
func New(cfg *core.VeteranAffairsConfig, opts Options) (VeteransService, error) {
	if cfg == nil {
		return nil, errors.New("cfg is required")
	}
	if cfg.TokenURL == "" {
		return nil, errors.New("cfg.TokenURL is required")
	}
	if cfg.TokenRecipientURL == "" {
		return nil, errors.New("cfg.TokenRecipientURL is required")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("cfg.ClientID is required")
	}
	if cfg.PrivateKeyPath == "" {
		return nil, errors.New("cfg.PrivateKeyPath is required")
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(
		slog.String("component", "veterans"),
		slog.String("vendor", "va"),
	)

	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &service{
		cfg:     cfg,
		client:  client,
		logger:  logger,
		timeout: timeout,
	}, nil
}

// GetAccessToken returns an OAuth access token using VA CCG.
func (s *service) GetAccessToken(ctx context.Context, icn string, scopes []string) (*AccessToken, error) {
	if icn == "" {
		return nil, errors.New("icn required")
	}
	if len(scopes) == 0 {
		return nil, errors.New("scopes can't be empty")
	}

	// Fast-path (no lock held long)
	if tok := s.getCachedTokenIfValid(); tok != nil {
		return tok, nil
	}

	// Ensure we have a timeout if caller didn't provide a deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	// Lock and re-check
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedToken != nil && time.Now().UTC().Before(s.tokenExpAt.Add(-tokenReuseSkewSeconds*time.Second)) {
		return s.cachedToken, nil
	}

	assertion, err := BuildClientAssertion(
		s.cfg.ClientID,
		s.cfg.PrivateKeyPath,
		s.cfg.TokenRecipientURL,
	)
	if err != nil {
		return nil, err
	}

	launch, err := BuildLaunchParam(icn)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("grant_type", grantType)
	form.Set("client_assertion_type", clientAssertionType)
	form.Set("client_assertion", assertion)
	form.Set("scope", strings.Join(scopes, " "))
	form.Set("launch", launch)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", acceptJSON)
	req.Header.Set("Content-Type", contentTypeForm)

	log := s.logger.With(
		slog.String("method", req.Method),
		slog.String("host", req.URL.Host),
		slog.String("path", req.URL.Path),
	)

	start := time.Now()
	resp, err := s.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		log.Error("va token request failed", slog.Any("error", err), slog.Duration("latency", latency))
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	log.Info("va token response received",
		slog.Int("status", resp.StatusCode),
		slog.String("content_type", resp.Header.Get("Content-Type")),
		slog.Duration("latency", latency),
	)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// IMPORTANT: never log assertion or token. Body could contain details; keep snippet small.
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 800 {
			snippet = snippet[:800] + "..."
		}
		log.Error("va token non-2xx",
			slog.Int("status", resp.StatusCode),
			slog.String("www_authenticate", resp.Header.Get("WWW-Authenticate")),
			slog.String("body_snippet", snippet),
		)
		return nil, fmt.Errorf("va token request failed: %s", resp.Status)
	}

	var out AccessToken
	if err := json.Unmarshal(body, &out); err != nil {
		log.Error("va token decode failed", slog.Any("error", err))
		return nil, err
	}
	if out.AccessToken == "" {
		return nil, errors.New("va token response missing access_token")
	}

	s.cachedToken = &out
	s.tokenExpAt = time.Now().UTC().Add(time.Duration(out.ExpiresIn) * time.Second)

	return &out, nil
}

func (s *service) getCachedTokenIfValid() *AccessToken {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedToken == nil {
		return nil
	}
	if time.Now().UTC().After(s.tokenExpAt.Add(-tokenReuseSkewSeconds * time.Second)) {
		return nil
	}
	return s.cachedToken
}

// Submit placeholder (you'll call VA disability-rating endpoint using Bearer token)
func (s *service) Submit(ctx context.Context, req DisabilityRatingRequest) (DisabilityRatingResponse_200, error) {
	return DisabilityRatingResponse_200{}, errors.New("not implemented")
}
