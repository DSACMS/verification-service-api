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

func (f *vaTokenFetcher) Fetch(ctx context.Context) (*oauthLocal.Token, error) {
	assertion, err := BuildClientAssertion(f.cfg.ClientID, f.cfg.PrivateKeyPath, f.cfg.TokenRecipientURL)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("grant_type", grantType)
	form.Set("client_assertion_type", clientAssertionType)
	form.Set("client_assertion", assertion)
	form.Set("scope", strings.Join(f.scopes, " "))

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
		return nil, fmt.Errorf("va token request failed: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}

	var at AccessToken
	if err := json.Unmarshal(body, &at); err != nil {
		return nil, err
	}
	if at.AccessToken == "" {
		return nil, errors.New("va token response missing access_token")
	}

	return &oauthLocal.Token{
		AccessToken: at.AccessToken,
		TokenType:   at.TokenType,
		Scope:       at.Scope,
		ExpiresIn:   at.ExpiresIn,
	}, nil
}
