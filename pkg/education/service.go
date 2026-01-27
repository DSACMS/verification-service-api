package education

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/DSACMS/verification-service-api/pkg/core"
)

type OAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

func OAuthTokenGenerator(ctx context.Context, cfg *core.NSCConfig) (OAuthTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", "vs.api.insights")
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		cfg.TokenURL,
		bytes.NewBufferString(form.Encode()),
	)
	if err != nil {
		return OAuthTokenResponse{}, fmt.Errorf("create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return OAuthTokenResponse{}, fmt.Errorf("send token request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return OAuthTokenResponse{}, fmt.Errorf("token request failed: status=%d body=%s", resp.StatusCode, string(raw))
	}

	var result OAuthTokenResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return OAuthTokenResponse{}, fmt.Errorf("decode token response: %w body=%s", err, string(raw))
	}

	if result.TokenType == "" {
		result.TokenType = "Bearer"
	}

	return result, nil
}

func TestEducationEndpoint(ctx context.Context, cfg *core.Config, reqBody Request) (Response, error) {
	// 1) Get token
	token, err := OAuthTokenGenerator(ctx, &cfg.NSC)
	if err != nil {
		return Response{}, fmt.Errorf("nsc oauth failed: %w\n\n", err)
	}
	if token.AccessToken == "" {
		return Response{}, fmt.Errorf("nsc oauth returned empty access token\n\n")
	}

	log.Printf(
		"token_type=%q\nexpires_in=%d\nscope=%q\naccess_token_len=%d\n\n",
		token.TokenType, token.ExpiresIn, token.Scope, len(token.AccessToken),
	)

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, fmt.Errorf("failed to marshal submit body: %w\n\n", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.NSC.SubmitURL, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("failed to create submit request: %w\n\n", err)
	}

	accToken := strings.TrimSpace(token.AccessToken)

	req.Header.Set("Authorization", "Bearer "+accToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	log.Printf("token url=%s\nsubmit url=%s\n\n", cfg.NSC.TokenURL, cfg.NSC.SubmitURL)

	authHeader := req.Header.Get("Authorization")
	if len(authHeader) > 24 {
		authHeader = authHeader[:24] + "..."
	}
	log.Printf("auth header=%q host=%q\n\n", authHeader, req.URL.Host)

	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) > 0 {
				r.Header = via[0].Header.Clone()
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("failed to submit request: %w\n\n", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return Response{}, fmt.Errorf(
			"nsc submit redirected: status=%d\nlocation=%q\nbody=%q\n\n",
			resp.StatusCode,
			resp.Header.Get("Location"),
			string(respBytes),
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf(
			"nsc submit failed: status=%d\nwww-authenticate=%q \ncontent-type=%q\nbody=%q\n\n",
			resp.StatusCode,
			resp.Header.Get("WWW-Authenticate"),
			resp.Header.Get("Content-Type"),
			string(respBytes),
		)
	}

	var result Response
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return Response{}, fmt.Errorf("decode nsc response: %w body=%q\n\n", err, string(respBytes))
	}

	return result, nil
}
