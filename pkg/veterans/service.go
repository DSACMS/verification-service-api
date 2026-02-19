package veterans

import (
	"bytes"
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
	grantType           string = "client_credentials"
	clientAssertionType string = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
	contentTypeForm     string = "application/x-www-form-urlencoded"
	applicationJSON     string = "application/json"

	tokenReuseSkewSeconds = 30 * time.Second

	authHeader = "Authorization"
)

var DefaultTokenScopes = []string{
	"launch",
	"veteran_status.read",
}

type VeteransService interface {
	GetAccessToken(ctx context.Context, scopes []string) (*AccessToken, error)
	Submit(ctx context.Context, icn string, req DisabilityRatingRequest) (DisabilityRatingResponse, error)
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
func (s *service) GetAccessToken(ctx context.Context, scopes []string) (*AccessToken, error) {
	if len(scopes) == 0 {
		return nil, errors.New("scopes can't be empty")
	}
	if s.cfg == nil {
		return nil, errors.New("missing config")
	}

	tok := s.getCachedTokenIfValid()
	if tok != nil {
		s.logger.Debug("VA TOKEN EXCHANGE (CACHE HIT)", slog.String("flow", "oauth_ccg"))
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

	if s.cachedToken != nil && time.Now().UTC().Before(s.tokenExpAt.Add(-tokenReuseSkewSeconds)) {
		s.logger.Debug("VA TOKEN EXCHANGE (CACHE HIT POST-LOCK)", slog.String("flow", "oauth_ccg"))
		return s.cachedToken, nil
	}

	assertion, err := BuildClientAssertion(s.cfg.ClientID, s.cfg.PrivateKeyPath, s.cfg.TokenRecipientURL)
	if err != nil {
		s.logger.Error("VA assertion build failed", slog.Any("error", err))
		return nil, err
	}

	form := url.Values{}
	form.Set("grant_type", grantType)
	form.Set("client_assertion_type", clientAssertionType)
	form.Set("client_assertion", assertion)
	form.Set("scope", strings.Join(scopes, " "))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		s.logger.Error("VA token request build failed", slog.Any("error", err))
		return nil, err
	}
	req.Header.Set("Accept", applicationJSON)
	req.Header.Set("Content-Type", contentTypeForm)

	start := time.Now()
	resp, err := s.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		s.logger.Error("VA token request failed", slog.Any("error", err), slog.Duration("latency", latency))
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody := strings.TrimSpace(string(body))
		if len(respBody) > 800 {
			respBody = respBody[:800] + "..."
		}
		s.logger.Error("VA token non-2xx",
			slog.Int("status", resp.StatusCode),
			slog.String("www_authenticate", resp.Header.Get("WWW-Authenticate")),
			slog.String("body_response", respBody),
			slog.Duration("latency", latency),
		)
		return nil, fmt.Errorf("va token request failed: status=%s body=%s", resp.Status, respBody)
	}

	var response AccessToken
	if err := json.Unmarshal(body, &response); err != nil {
		s.logger.Error("VA token decode failed", slog.Any("error", err))
		return nil, err
	}
	if response.AccessToken == "" {
		return nil, errors.New("va token response missing access_token")
	}

	s.cachedToken = &response
	s.tokenExpAt = time.Now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second)

	s.logger.Debug("VA token cached", slog.Int("expires_in", response.ExpiresIn), slog.Time("token_exp_at_utc", s.tokenExpAt))
	return &response, nil
}

func (s *service) getCachedTokenIfValid() *AccessToken {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedToken == nil {
		return nil
	}
	if time.Now().UTC().After(s.tokenExpAt.Add(-tokenReuseSkewSeconds)) {
		return nil
	}
	return s.cachedToken
}

// Submit calls the VA Disability Rating endpoint using:
// - launch built from ICN (request-scoped)
// - OAuth access token (client-scoped)
func (s *service) Submit(ctx context.Context, icn string, req DisabilityRatingRequest) (DisabilityRatingResponse, error) {
	if icn == "" {
		return DisabilityRatingResponse{}, errors.New("icn required")
	}

	if s.cfg == nil {
		return DisabilityRatingResponse{}, errors.New("missing config")
	}

	if s.cfg.DisabilityRatingURL == "" {
		return DisabilityRatingResponse{}, errors.New("cfg.DisabilityRatingURL is required")
	}

	if len(DefaultTokenScopes) == 0 {
		return DisabilityRatingResponse{}, errors.New("defaultVAScopes must be set")
	}

	launch, err := BuildLaunchParam(icn)
	if err != nil {
		s.logger.Error("VA launch param build failed", slog.Any("error", err))
		return DisabilityRatingResponse{}, err
	}

	_, hasDeadline := ctx.Deadline()
	if !hasDeadline && s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	tok, err := s.GetAccessToken(ctx, DefaultTokenScopes)
	if err != nil {
		return DisabilityRatingResponse{}, err
	}

	endpoint, err := url.Parse(s.cfg.DisabilityRatingURL)
	if err != nil {
		return DisabilityRatingResponse{}, fmt.Errorf("invalid DisabilityRatingURL: %w", err)
	}

	q := endpoint.Query()
	q.Set("launch", launch)
	endpoint.RawQuery = q.Encode()

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return DisabilityRatingResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return DisabilityRatingResponse{}, err
	}

	httpReq.Header.Set("Accept", applicationJSON)
	httpReq.Header.Set("Content-Type", applicationJSON)
	httpReq.Header.Set(authHeader, "Bearer "+tok.AccessToken)

	log := s.logger.With(
		slog.String("operation", "va_submit"),
		slog.String("method", httpReq.Method),
		slog.String("host", httpReq.URL.Host),
		slog.String("path", httpReq.URL.Path),
	)

	log.Debug("VA submit HTTP request start")

	start := time.Now()
	resp, err := s.client.Do(httpReq)
	latency := time.Since(start)
	if err != nil {
		log.Error("VA submit request failed", slog.Any("error", err), slog.Duration("latency", latency))
		return DisabilityRatingResponse{}, err
	}
	defer resp.Body.Close()

	respBodyBytes, _ := io.ReadAll(resp.Body)

	log.Info("VA submit response received",
		slog.Int("status", resp.StatusCode),
		slog.String("content_type", resp.Header.Get("Content-Type")),
		slog.Duration("latency", latency),
	)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(respBodyBytes))
		if len(snippet) > 800 {
			snippet = snippet[:800] + "..."
		}

		log.Error("VA submit non-2xx",
			slog.Int("status", resp.StatusCode),
			slog.String("www_authenticate", resp.Header.Get("WWW-Authenticate")),
			slog.String("body_response", snippet),
		)

		return DisabilityRatingResponse{}, fmt.Errorf("va submit failed: status=%s body=%s", resp.Status, snippet)
	}

	var out DisabilityRatingResponse
	if err := json.Unmarshal(respBodyBytes, &out); err != nil {
		log.Error("VA submit decode failed", slog.Any("error", err))
		return DisabilityRatingResponse{}, err
	}

	log.Debug("VA submit end (OK)")
	return out, nil
}
