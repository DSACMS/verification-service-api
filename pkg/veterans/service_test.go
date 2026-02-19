package veterans

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DSACMS/verification-service-api/pkg/core"
)

func writeTempPEMKey(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}

	f, err := os.CreateTemp(t.TempDir(), "private-*.pem")
	if err != nil {
		t.Fatalf("create temp pem: %v", err)
	}
	err = pem.Encode(f, block)
	if err != nil {
		t.Fatalf("encode pem: %v", err)
	}
	_ = f.Close()
	return f.Name()
}

func TestGetAccessToken_HappyPath_AndCaching(t *testing.T) {
	var hits int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)

		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("expected Accept application/json, got %q", got)
		}
		if got := r.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-www-form-urlencoded") {
			t.Fatalf("expected form content-type, got %q", got)
		}

		bodyBytes, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(bodyBytes))
		if err != nil {
			t.Fatalf("parse form: %v", err)
		}

		// passing in required params
		if form.Get("grant_type") != "client_credentials" {
			t.Fatalf("grant_type mismatch: %q", form.Get("grant_type"))
		}
		if form.Get("client_assertion_type") != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" {
			t.Fatalf("client_assertion_type mismatch: %q", form.Get("client_assertion_type"))
		}
		if form.Get("client_assertion") == "" {
			t.Fatalf("missing client_assertion")
		}
		if form.Get("launch") == "" {
			t.Fatalf("missing launch")
		}
		if form.Get("scope") != "disability-rating.read something.else" {
			t.Fatalf("scope mismatch: %q", form.Get("scope"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"access_token": "test-token-123",
			"token_type": "Bearer",
			"scope": "disability-rating.read",
			"expires_in": 3600
		}`))
	}))
	defer srv.Close()

	pemPath := writeTempPEMKey(t)

	cfg := &core.VeteranAffairsConfig{
		ClientID:          "client-123",
		PrivateKeyPath:    pemPath,
		TokenRecipientURL: "https://deptva-eval.okta.com/oauth2/someid/v1/token",
		TokenURL:          srv.URL,
	}

	svc, err := New(cfg, Options{Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx := context.Background()
	scopes := []string{"disability-rating.read", "something.else"}

	// should hit server
	tok1, err := svc.GetAccessToken(ctx, scopes)
	if err != nil {
		t.Fatalf("GetAccessToken #1: %v", err)
	}
	if tok1.AccessToken != "test-token-123" {
		t.Fatalf("unexpected token: %q", tok1.AccessToken)
	}

	// should use cache without hitting server
	tok2, err := svc.GetAccessToken(ctx, scopes)
	if err != nil {
		t.Fatalf("GetAccessToken #2: %v", err)
	}
	if tok2.AccessToken != "test-token-123" {
		t.Fatalf("unexpected token #2: %q", tok2.AccessToken)
	}

	got := atomic.LoadInt32(&hits)
	if got != 1 {
		t.Fatalf("expected 1 HTTP hit due to caching, got %d", got)
	}
}
