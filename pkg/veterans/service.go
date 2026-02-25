package veterans

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
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

type VeteransService interface {
	GetAccessToken(ctx context.Context, icn string, scopes []string) (*AccessToken, error)
	Submit(ctx context.Context, icn string, req DisabilityRatingRequest) (DisabilityRatingResponse, error)
	GetDisabilityRating(ctx context.Context, icn string, req DisabilityRatingRequest) (DisabilityRatingResponse, error)
}

type HTTPTransport interface {
	Do(req *http.Request) (*http.Response, error)
}

type Options struct {
	HTTPClient HTTPTransport
	Logger     *slog.Logger
	Timeout    time.Duration
}

type service struct {
	cfg     *core.VeteranAffairsConfig
	client  HTTPTransport
	logger  *slog.Logger
	timeout time.Duration

	mu        sync.Mutex
	tokenSrcs map[string]oauth2.TokenSource
}

type AccessToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
}

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
		timeout = optsTimeout
	}

	return &service{
		cfg:       cfg,
		client:    client,
		logger:    logger,
		timeout:   timeout,
		tokenSrcs: make(map[string]oauth2.TokenSource),
	}, nil
}

func (s *service) GetDisabilityRating(
	ctx context.Context,
	icn string,
	req DisabilityRatingRequest,
) (DisabilityRatingResponse, error) {
	var zero DisabilityRatingResponse

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

	tok, err := s.GetAccessToken(ctx, icn, scopes)
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

func (s *service) GetAccessToken(ctx context.Context, icn string, scopes []string) (*AccessToken, error) {
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
		slog.String("access_token_prefix", prefix(tok.AccessToken, 12)),
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
	s.logger.Info("VA token request inputs",
		slog.String("client_id_prefix", prefix(s.cfg.ClientID, 8)),
		slog.String("okta_aud", s.cfg.TokenRecipientURL),
		slog.String("token_url", s.cfg.TokenURL),
		slog.String("key_path", s.cfg.PrivateKeyPath),
		slog.String("scope", scopeStr),
	)

	keyBytes, err := os.ReadFile(s.cfg.PrivateKeyPath)
	if err != nil {
		s.logger.Error("VA private key read failed",
			slog.String("key_path", s.cfg.PrivateKeyPath),
			slog.Any("error", err),
		)
		return nil, err
	}

	firstLine := strings.SplitN(string(keyBytes), "\n", 2)[0]

	modHash, mhErr := rsaModulusMD5FromPEM(keyBytes)

	if mhErr != nil {
		s.logger.Warn("VA private key parse failed (fingerprint unavailable)",
			slog.String("first_line", firstLine),
			slog.Any("error", mhErr),
		)
	} else {
		s.logger.Info("VA private key fingerprint",
			slog.String("first_line", firstLine),
			slog.String("rsa_modulus_md5", modHash),
		)
	}

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

	ctx := context.Background()
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
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

	return &oauth2.Token{
		AccessToken: response.AccessToken,
		TokenType:   response.TokenType,
		Expiry:      exp,
	}, nil
}

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

	if _, hasDeadline := ctx.Deadline(); !hasDeadline && s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	tok, err := s.GetAccessToken(ctx, icn, DefaultTokenScopes)
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

func rsaModulusMD5FromPEM(pemBytes []byte) (string, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return "", errors.New("failed to decode PEM block")
	}

	var priv any
	var err error

	if block.Type == "RSA PRIVATE KEY" {
		priv, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return "", err
		}
	} else {
		priv, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return "", err
		}
	}

	rsaPriv, ok := priv.(*rsa.PrivateKey)
	if !ok {
		return "", errors.New("not an RSA private key")
	}

	sum := md5.Sum(rsaPriv.N.Bytes())
	return hex.EncodeToString(sum[:]), nil
}
