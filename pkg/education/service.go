package education

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/DSACMS/verification-service-api/pkg/core"
	"github.com/gofiber/fiber/v2"
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
	if resp.StatusCode != fiber.StatusOK {
		return OAuthTokenResponse{}, fmt.Errorf("token request failed: status=%d body=%s", resp.StatusCode, string(raw))
	}

	var result OAuthTokenResponse
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return OAuthTokenResponse{}, fmt.Errorf("decode token response: %w body=%s", err, string(raw))
	}

	return result, nil
}

func TestEducationEndpoint(ctx context.Context, cfg *core.Config, reqBody Request) (Response, error) {
	token, err := OAuthTokenGenerator(ctx, &cfg.NSC)
	if err != nil {
		return Response{}, err
	}

	// auth := token.TokenType + " " + token.AccessToken
	auth := "Bearer " + token.AccessToken

	url := cfg.NSC.BaseURL

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Response{}, fmt.Errorf("marshal submit body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create submit request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", auth)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("send submit request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != fiber.StatusOK {
		return Response{}, fmt.Errorf("failed o read response body. got error: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, fmt.Errorf("nsc returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var result Response

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return Response{}, fmt.Errorf("failed to unmarshal. got error: %w", err)
	}

	return result, nil
}
