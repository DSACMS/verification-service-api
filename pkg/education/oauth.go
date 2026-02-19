package education

import (
	"context"
	"net/http"

	"github.com/DSACMS/verification-service-api/pkg/core"
	"github.com/DSACMS/verification-service-api/pkg/oauthLocal"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func nscHTTPClient(ctx context.Context, cfg *core.NSCConfig) *http.Client {
	base := oauthLocal.HeaderPreservingClient()

	ctx = context.WithValue(ctx, oauth2.HTTPClient, base)

	cc := &clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
		Scopes:       []string{"vs.api.insights"},
	}

	return oauth2.NewClient(ctx, cc.TokenSource(ctx))
}
