package oauthLocal

import (
	"context"
	"sync"
	"time"
)

type Token struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int
	Scope       string
}

type TokenFetcher interface {
	Fetch(ctx context.Context) (*Token, error)
}

type CachedFetcher struct {
	fetcher   TokenFetcher
	skew      time.Duration
	mu        sync.Mutex
	token     *Token
	expiresAt time.Time
}

func NewCachedFetcher(f TokenFetcher, skew time.Duration) *CachedFetcher {
	return &CachedFetcher{fetcher: f, skew: skew}
}

func (c *CachedFetcher) Token(ctx context.Context) (*Token, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != nil && time.Now().UTC().Before(c.expiresAt.Add(-c.skew)) {
		return c.token, nil
	}

	tok, err := c.fetcher.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	c.token = tok
	c.expiresAt = time.Now().UTC().Add(time.Duration(tok.ExpiresIn) * time.Second)
	return tok, nil
}
