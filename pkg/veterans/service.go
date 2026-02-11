package veterans

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/DSACMS/verification-service-api/pkg/core"
)

const (
	contextTimeout time.Duration = 10 * time.Second

	grantType             = "client_credentials"
	clientAssertionType   = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
	contentTypeForm       = "application/x-www-form-urlencoded"
	acceptJSON            = "application/json"
	tokenReuseSkewSeconds = 30 // safety buffer so we refresh a bit early
)

type VeteransService interface {
	Submit(ctx context.Context, req DisabilityRatingRequest) (DisabilityRatingResponse_200, error)

	GetAccessToken(ctx context.Context, icn string, scopes []string) (*AccessToken, error)
}

type service struct {
	cfg    *core.VeteranAffairsConfig
	client HTTPTransport

	mu          sync.Mutex
	cachedToken *AccessToken
	tokenExpAt  time.Time
}

type HTTPTransport interface {
	Do(req *http.Request) (*http.Response, error)
}

type AccessToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
}

// New creates the veterans service.
func New(cfg *core.VeteranAffairsConfig, client HTTPTransport) (VeteransService, error) {
	if cfg == nil {
		return nil, errors.New("cfg is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &service{cfg: cfg, client: client}, nil
}

// Returns an OAuth access token using VA CCG.
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

	if tok := s.getCachedTokenIfValid(); tok != nil {
		return tok, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after getting lock
	if s.cachedToken != nil && time.Now().UTC().Before(s.tokenExpAt.Add(-tokenReuseSkewSeconds*time.Second)) {
		return s.cachedToken, nil
	}

	ctx, cancel := context.WithTimeout(ctx, contextTimeout)
	defer cancel()

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

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// IMPORTANT: do not include assertion/token in logs.
		return nil, fmt.Errorf("va token request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var out AccessToken
	err = json.Unmarshal(body, &out)
	if err != nil {
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

// Submit is a placeholder; keep your actual implementation.
//
// Typically Submit would:
// 1) call GetAccessToken(ctx, req.ICN, desiredScopes)
// 2) call the VA downstream endpoint with Authorization: Bearer <token>
func (s *service) Submit(ctx context.Context, req DisabilityRatingRequest) (DisabilityRatingResponse_200, error) {
	return DisabilityRatingResponse_200{}, errors.New("not implemented")
}
