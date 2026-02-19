package oauthLocal

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func ClientCredentialsHTTPClient(ctx context.Context, cc *clientcredentials.Config, base *http.Client) *http.Client {
	if base == nil {
		base = http.DefaultClient
	}

	ctx = WithBaseClient(ctx, base)
	return oauth2.NewClient(ctx, cc.TokenSource(ctx))
}
