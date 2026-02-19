package oauthLocal

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

func HeaderPreservingClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) > 0 {
				r.Header = via[0].Header.Clone()
			}

			return nil
		},
	}
}

func WithBaseClient(ctx context.Context, base *http.Client) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, base)
}
