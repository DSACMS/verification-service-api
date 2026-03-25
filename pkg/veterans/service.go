package veterans

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
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
	"golang.org/x/oauth2"
)

const (
	grantType           string = "client_credentials"
	clientAssertionType string = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
	contentTypeForm     string = "application/x-www-form-urlencoded"
	applicationJSON     string = "application/json"

	tokenReuseSkewSeconds time.Duration = 30 * time.Second
	authHeader            string        = "Authorization"
	optsTimeout           time.Duration = 10 * time.Second

	maxErrBodyLogBytes = 800
)

var DefaultTokenScopes = []string{
	"launch",
	"veteran_status.read",
}

type RedisCache interface {
	Get(ctx context.Context, key string) (string, error)
	SetEX(ctx context.Context, key string, value string, ttl time.Duration) error
}

type cachedToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiryUnix  int64  `json:"expiry_unix"` // unix seconds
	Scope       string `json:"scope"`
}

type VeteransService interface {
	GetDisabilityRating(ctx context.Context, icn string, req DisabilityRatingRequest) (DisabilityRatingResponse, error)
}

type HTTPTransport interface {
	Do(req *http.Request) (*http.Response, error)
}

type Options struct {
	HTTPClient HTTPTransport
	Logger     *slog.Logger
	Timeout    time.Duration

	Redis RedisCache
}

type service struct {
	cfg     *core.VeteranAffairsConfig
	client  HTTPTransport
	logger  *slog.Logger
	timeout time.Duration

	mu        sync.Mutex
	tokenSrcs map[string]oauth2.TokenSource

	redis RedisCache
}

type AccessToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
}

func New(cfg *core.VeteranAffairsConfig, opts Options) (*service, error) {
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
		timeout = optsTimeout
	}

	return &service{
		cfg:       cfg,
		client:    client,
		logger:    logger,
		timeout:   timeout,
		redis:     opts.Redis,
		tokenSrcs: make(map[string]oauth2.TokenSource),
	}, nil
}

func (s *service) GetDisabilityRating(
	ctx context.Context,
	icn string,
	req DisabilityRatingRequest,
) (DisabilityRatingResponse, error) {
	var zero DisabilityRatingResponse

	icn = normalizeICN(icn)
	if icn == "" {
		return zero, errors.New("icn required")
	}
	if s.cfg == nil {
		return zero, errors.New("missing config")
	}
	if strings.TrimSpace(s.cfg.DisabilityRatingURL) == "" {
		return zero, errors.New("cfg.DisabilityRatingURL is required")
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline && s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	scopes := []string{"launch", "disability_rating.read"}

	tok, err := s.getAccessToken(ctx, icn, scopes)
	if err != nil {
		return zero, err
	}

	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return zero, err
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.cfg.DisabilityRatingURL,
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return zero, err
	}

	httpReq.Header.Set("Accept", applicationJSON)
	httpReq.Header.Set("Content-Type", applicationJSON)

	// VA gateway headers for the resource call (not the token call)
	if strings.TrimSpace(s.cfg.SandboxKey) != "" {
		httpReq.Header.Set("apikey", strings.TrimSpace(s.cfg.SandboxKey))
	}
	if strings.TrimSpace(s.cfg.SandboxRequestID) != "" {
		httpReq.Header.Set("X-VA-Request-Id", strings.TrimSpace(s.cfg.SandboxRequestID))
	}

	tokenType := strings.TrimSpace(tok.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	httpReq.Header.Set(authHeader, tokenType+" "+tok.AccessToken)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, fmt.Errorf(
			"va disability_rating failed: status=%s body=%s",
			resp.Status,
			strings.TrimSpace(string(respBody)),
		)
	}

	var out DisabilityRatingResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return zero, err
	}

	return out, nil
}

func (s *service) getAccessToken(ctx context.Context, icn string, scopes []string) (*AccessToken, error) {
	icn = normalizeICN(icn)
	if icn == "" {
		return nil, errors.New("icn is required")
	}
	if len(scopes) == 0 {
		return nil, errors.New("scopes can't be empty")
	}
	if s.cfg == nil {
		return nil, errors.New("missing config")
	}

	launch, err := BuildLaunchParam(icn)
	if err != nil {
		return nil, err
	}

	_, hasDeadline := ctx.Deadline()
	if !hasDeadline && s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	scopeKey := strings.Join(scopes, " ")
	cacheKey := launch + "|" + scopeKey

	tokenSource := s.getOrCreateTokenSource(cacheKey, scopes, launch)

	start := time.Now()
	tok, err := tokenSource.Token()
	latency := time.Since(start)

	if err != nil {
		s.logger.Error("VA token request failed",
			slog.Any("error", err),
			slog.Duration("latency", latency),
			slog.String("scope", scopeKey),
		)
		return nil, err
	}
	if tok == nil {
		s.logger.Error("VA token response was nil",
			slog.Duration("latency", latency),
			slog.String("scope", scopeKey),
		)
		return nil, errors.New("va token response was nil")
	}

	expiresIn := int(time.Until(tok.Expiry).Seconds())

	s.logger.Info("VA token acquired",
		slog.String("token_type", tok.TokenType),
		slog.String("scope", scopeKey),
		slog.Int("expires_in", expiresIn),
		slog.Duration("latency", latency),
	)

	return &AccessToken{
		AccessToken: tok.AccessToken,
		TokenType:   tok.TokenType,
		Scope:       scopeKey,
		ExpiresIn:   expiresIn,
	}, nil
}

func (s *service) getOrCreateTokenSource(cacheKey string, scopes []string, launch string) oauth2.TokenSource {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ts, ok := s.tokenSrcs[cacheKey]; ok {
		return ts
	}

	base := &vaTokenSource{
		cfg:     s.cfg,
		client:  s.client,
		logger:  s.logger,
		scopes:  scopes,
		launch:  launch,
		timeout: s.timeout,
		redis:   s.redis,
	}

	reuse := oauth2.ReuseTokenSourceWithExpiry(nil, base, tokenReuseSkewSeconds)
	s.tokenSrcs[cacheKey] = reuse
	return reuse
}

type vaTokenSource struct {
	cfg     *core.VeteranAffairsConfig
	client  HTTPTransport
	logger  *slog.Logger
	scopes  []string
	launch  string
	timeout time.Duration
	redis   RedisCache
}

func (s *vaTokenSource) Token() (*oauth2.Token, error) {
	if s.cfg == nil {
		return nil, errors.New("missing config")
	}
	if s.client == nil {
		return nil, errors.New("http client missing")
	}
	if s.logger == nil {
		return nil, errors.New("logger missing")
	}
	if len(s.scopes) == 0 {
		return nil, errors.New("scopes can't be empty")
	}
	if s.launch == "" {
		return nil, errors.New("launch is required")
	}

	if strings.TrimSpace(s.cfg.ClientID) == "" {
		return nil, errors.New("cfg.ClientID is required")
	}
	if strings.TrimSpace(s.cfg.PrivateKeyPath) == "" {
		return nil, errors.New("cfg.PrivateKeyPath is required")
	}
	if strings.TrimSpace(s.cfg.TokenRecipientURL) == "" {
		return nil, errors.New("cfg.TokenRecipientURL (JWT aud) is required")
	}
	if strings.TrimSpace(s.cfg.TokenURL) == "" {
		return nil, errors.New("cfg.TokenURL is required")
	}

	scopeStr := strings.Join(s.scopes, " ")

	ctx := context.Background()
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	if s.redis != nil {
		cacheKey := s.redisKey(scopeStr)

		raw, err := s.redis.Get(ctx, cacheKey)
		if err == nil && strings.TrimSpace(raw) != "" {
			var ct cachedToken
			if jerr := json.Unmarshal([]byte(raw), &ct); jerr == nil && ct.AccessToken != "" {
				exp := time.Unix(ct.ExpiryUnix, 0).UTC()
				if tokenStillUsable(exp) {
					return &oauth2.Token{
						AccessToken: ct.AccessToken,
						TokenType:   ct.TokenType,
						Expiry:      exp,
					}, nil
				}
			}
		}
	}

	s.logger.Info("VA token request",
		slog.String("token_url", s.cfg.TokenURL),
		slog.String("scope", scopeStr),
	)

	assertion, err := GetAssertionPrivatekey(
		s.cfg.ClientID,
		s.cfg.PrivateKeyPath,
		s.cfg.TokenRecipientURL,
	)
	if err != nil {
		s.logger.Error("VA assertion build failed", slog.Any("error", err))
		return nil, err
	}

	form := url.Values{
		"grant_type":            {grantType},
		"client_assertion_type": {clientAssertionType},
		"client_assertion":      {assertion},
		"scope":                 {scopeStr},
		"launch":                {s.launch},
	}

	req, err := newFormPOST(ctx, http.MethodPost, s.cfg.TokenURL, form)
	if err != nil {
		s.logger.Error("VA token request build failed", slog.Any("error", err))
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody := strings.TrimSpace(string(body))
		if len(respBody) > maxErrBodyLogBytes {
			respBody = respBody[:maxErrBodyLogBytes] + "..."
		}
		return nil, fmt.Errorf("va token request failed: status=%s body=%s", resp.Status, respBody)
	}

	var response AccessToken
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.AccessToken == "" {
		return nil, errors.New("va token response missing access_token")
	}

	exp := time.Now().UTC().Add(time.Duration(response.ExpiresIn) * time.Second)

	tok := &oauth2.Token{
		AccessToken: response.AccessToken,
		TokenType:   response.TokenType,
		Expiry:      exp,
	}

	if s.redis != nil {
		ttl := time.Until(exp) - tokenReuseSkewSeconds
		if ttl < 5*time.Second {
			ttl = 5 * time.Second
		}

		ct := cachedToken{
			AccessToken: tok.AccessToken,
			TokenType:   tok.TokenType,
			ExpiryUnix:  tok.Expiry.Unix(),
			Scope:       scopeStr,
		}

		b, _ := json.Marshal(ct)
		cacheKey := s.redisKey(scopeStr)

		if err := s.redis.SetEX(ctx, cacheKey, string(b), ttl); err != nil {
			s.logger.Warn("VA token cache write failed", slog.Any("error", err))
		}
	}

	return tok, nil
}
func (s *service) Submit(ctx context.Context, icn string, req DisabilityRatingRequest) (DisabilityRatingResponse, error) {
	icn = normalizeICN(icn)
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

	if _, hasDeadline := ctx.Deadline(); !hasDeadline && s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	tok, err := s.getAccessToken(ctx, icn, DefaultTokenScopes)
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

	tokenType := strings.TrimSpace(tok.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	httpReq.Header.Set(authHeader, tokenType+" "+tok.AccessToken)

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return DisabilityRatingResponse{}, err
	}
	defer resp.Body.Close()

	respBodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(respBodyBytes))
		if len(snippet) > maxErrBodyLogBytes {
			snippet = snippet[:maxErrBodyLogBytes] + "..."
		}
		return DisabilityRatingResponse{}, fmt.Errorf("va submit failed: status=%s body=%s", resp.Status, snippet)
	}

	var out DisabilityRatingResponse
	if err := json.Unmarshal(respBodyBytes, &out); err != nil {
		return DisabilityRatingResponse{}, err
	}

	return out, nil
}

func prefix(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func newFormPOST(ctx context.Context, method, endpoint string, form url.Values) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", applicationJSON)
	req.Header.Set("Content-Type", contentTypeForm)
	return req, nil
}

func (s *vaTokenSource) redisKey(scopeStr string) string {
	sum := md5.Sum([]byte(s.launch + "|" + scopeStr))
	return "va:token:" + hex.EncodeToString(sum[:])
}

func tokenStillUsable(exp time.Time) bool {
	return time.Until(exp) > tokenReuseSkewSeconds
}
