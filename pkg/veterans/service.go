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
	if s.cfg == nil {
		return nil, errors.New("missing config")
	}

	tok := s.getCachedTokenIfValid()
	if tok != nil {
		s.logger.Debug("VA TOKEN EXCHANGE (CACHE HIT)",
			slog.String("component", "veterans"),
			slog.String("flow", "oauth_ccg"),
		)
		return tok, nil
	}

	_, hasDeadline := ctx.Deadline()
	if !hasDeadline && s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedToken != nil && time.Now().UTC().Before(s.tokenExpAt.Add(-tokenReuseSkewSeconds*time.Second)) {
		s.logger.Debug("VA TOKEN EXCHANGE (CACHE HIT POST-LOCK)",
			slog.String("component", "veterans"),
			slog.String("flow", "oauth_ccg"),
		)
		return s.cachedToken, nil
	}

	s.logger.Debug("VA TOKEN EXCHANGE START",
		slog.String("component", "veterans"),
		slog.String("flow", "oauth_ccg"),
	)

	assertion, err := BuildClientAssertion(
		s.cfg.ClientID,
		s.cfg.PrivateKeyPath,
		s.cfg.TokenRecipientURL,
	)
	if err != nil {
		s.logger.Error("VA assertion build failed",
			slog.String("component", "veterans"),
			slog.Any("error", err),
		)
		return nil, err
	}

	audHost := ""
	u, parseErr := url.Parse(s.cfg.TokenRecipientURL)
	if parseErr == nil && u != nil {
		audHost = u.Host
	}

	s.logger.Debug("VA assertion built",
		slog.Int("assertion_len", len(assertion)),
		slog.String("aud_host", audHost),
		slog.Int("scopes_count", len(scopes)),
	)

	launch, err := BuildLaunchParam(icn)
	if err != nil {
		s.logger.Error("VA launch param build failed",
			slog.String("component", "veterans"),
			slog.Any("error", err),
		)
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
		s.logger.Error("VA token request build failed",
			slog.String("component", "veterans"),
			slog.Any("error", err),
		)
		return nil, err
	}
	req.Header.Set("Accept", acceptJSON)
	req.Header.Set("Content-Type", contentTypeForm)

	log := s.logger.With(
		slog.String("component", "veterans"),
		slog.String("operation", "va_token"),
		slog.String("method", req.Method),
		slog.String("host", req.URL.Host),
		slog.String("path", req.URL.Path),
	)

	log.Debug("VA token HTTP request start")

	start := time.Now()
	resp, err := s.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		log.Error("VA token request failed",
			slog.Any("error", err),
			slog.Duration("latency", latency),
		)
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	log.Info("VA token response received",
		slog.Int("status", resp.StatusCode),
		slog.String("content_type", resp.Header.Get("Content-Type")),
		slog.Duration("latency", latency),
	)

	// non 200 status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody := strings.TrimSpace(string(body))
		// prevent overly long log with limit and ellipsis
		if len(respBody) > 800 {
			respBody = respBody[:800] + "..."
		}

		log.Error("VA token non-2xx",
			slog.Int("status", resp.StatusCode),
			slog.String("www_authenticate", resp.Header.Get("WWW-Authenticate")),
			slog.String("body_response", respBody),
		)

		s.logger.Debug("VA token exhange failed",
			slog.String("component", "veterans"),
			slog.Int("status", resp.StatusCode),
		)

		return nil, fmt.Errorf("va token request failed: status=%s body=%s", resp.Status, respBody)
	}

	var response AccessToken
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Error("VA token decode failed", slog.Any("error", err))

		s.logger.Debug("VA TOKEN EXCHANGE END (FAIL)",
			slog.String("component", "veterans"),
			slog.String("reason", "decode_failed"),
		)

		return nil, err
	}
	if response.AccessToken == "" {
		err := errors.New("va token response missing access_token")
		log.Error("VA token response invalid", slog.Any("error", err))

		s.logger.Debug("VA TOKEN EXCHANGE END (FAIL)",
			slog.String("component", "veterans"),
			slog.String("reason", "missing_access_token"),
		)

		return nil, err
	}

	s.cachedToken = &response
	s.tokenExpAt = time.Now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second)

	s.logger.Debug("VA token cached",
		slog.Int("expires_in", response.ExpiresIn),
		slog.Time("token_exp_at_utc", s.tokenExpAt),
	)

	s.logger.Debug("VA TOKEN EXCHANGE END (OK)",
		slog.String("component", "veterans"),
	)

	return &response, nil
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

func (s *service) Submit(ctx context.Context, req DisabilityRatingRequest) (DisabilityRatingResponse_200, error) {
	return DisabilityRatingResponse_200{}, errors.New("not implemented")
}
