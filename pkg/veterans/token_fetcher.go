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

	"github.com/DSACMS/verification-service-api/pkg/core"
	"github.com/DSACMS/verification-service-api/pkg/oauthLocal"
)

type vaTokenFetcher struct {
	cfg    *core.VeteranAffairsConfig
	client HTTPTransport
	logger *slog.Logger
	scopes []string
}

func (f *vaTokenFetcher) Fetch(ctx context.Context, icn string) (*oauthLocal.Token, error) {
	if f.cfg == nil {
		return nil, errors.New("missing config")
	}
	if f.client == nil {
		return nil, errors.New("http client missing")
	}
	if f.logger == nil {
		f.logger = slog.Default()
	}
	if icn == "" {
		return nil, errors.New("icn is required")
	}
	if len(f.scopes) == 0 {
		return nil, errors.New("scopes can't be empty")
	}

	// Required VA OAuth config
	if strings.TrimSpace(f.cfg.ClientID) == "" {
		return nil, errors.New("cfg.ClientID is required")
	}
	if strings.TrimSpace(f.cfg.PrivateKeyPath) == "" {
		return nil, errors.New("cfg.PrivateKeyPath is required")
	}
	if strings.TrimSpace(f.cfg.TokenRecipientURL) == "" {
		return nil, errors.New("cfg.TokenRecipientURL (JWT aud) is required")
	}
	if strings.TrimSpace(f.cfg.TokenURL) == "" {
		return nil, errors.New("cfg.TokenURL is required")
	}

	scopeStr := strings.Join(f.scopes, " ")

	// Key debug log for "signature invalid" issues (no secrets).
	f.logger.Info("VA token fetch inputs",
		slog.String("client_id_prefix", prefix(f.cfg.ClientID, 8)),
		slog.String("okta_aud", f.cfg.TokenRecipientURL),
		slog.String("token_url", f.cfg.TokenURL),
		slog.String("key_path", f.cfg.PrivateKeyPath),
		slog.String("scope", scopeStr),
	)

	launch, err := BuildLaunchParam(icn)
	if err != nil {
		return nil, err
	}

	assertion, err := GetAssertionPrivatekey(
		f.cfg.ClientID,
		f.cfg.PrivateKeyPath,
		f.cfg.TokenRecipientURL,
	)
	if err != nil {
		f.logger.Error("VA assertion build failed", slog.Any("error", err))
		return nil, err
	}

	form := url.Values{
		"grant_type":            {grantType},
		"client_assertion_type": {clientAssertionType},
		"client_assertion":      {assertion},
		"scope":                 {scopeStr},
		"launch":                {launch},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", applicationJSON)
	req.Header.Set("Content-Type", contentTypeForm)

	resp, err := f.client.Do(req)
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

	var at AccessToken
	if err := json.Unmarshal(body, &at); err != nil {
		return nil, err
	}
	if at.AccessToken == "" {
		return nil, errors.New("va token response missing access_token")
	}

	// Prefer the server-returned scope if present; otherwise keep what we requested.
	scopeOut := strings.TrimSpace(at.Scope)
	if scopeOut == "" {
		scopeOut = scopeStr
	}

	return &oauthLocal.Token{
		AccessToken: at.AccessToken,
		TokenType:   at.TokenType,
		Scope:       scopeOut,
		ExpiresIn:   at.ExpiresIn,
	}, nil
}
