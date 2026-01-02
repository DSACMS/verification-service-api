package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRSAKeyPair(t *testing.T) (*rsa.PrivateKey, jwk.Key) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pubJWK, err := jwk.FromRaw(&priv.PublicKey)
	require.NoError(t, err)

	require.NoError(t, pubJWK.Set(jwk.KeyIDKey, "test-kid"))

	return priv, pubJWK
}

func newJWKSserver(t *testing.T, set jwk.Set) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(set))
	}))
}

func newFailingJWKSserver() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
}

func signAccessToken(t *testing.T, priv *rsa.PrivateKey, issuer, clientID string, extraClaims map[string]any) string {
	t.Helper()

	tok := jwt.New()
	require.NoError(t, tok.Set(jwt.IssuerKey, issuer))
	require.NoError(t, tok.Set("token_use", "access"))
	require.NoError(t, tok.Set("client_id", clientID))

	require.NoError(t, tok.Set("sub", "user-123"))
	require.NoError(t, tok.Set("username", "imhotep"))
	require.NoError(t, tok.Set("scope", "read:all"))
	require.NoError(t, tok.Set("cognito:groups", []string{"admins"}))

	for k, v := range extraClaims {
		require.NoError(t, tok.Set(k, v))
	}

	hdrs := jws.NewHeaders()
	require.NoError(t, hdrs.Set(jws.KeyIDKey, "test-kid"))

	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, priv, jws.WithProtectedHeaders(hdrs)))
	require.NoError(t, err)

	return string(signed)
}

func makeAppWithMiddleware(v *CognitoVerifier) *fiber.App {
	app := fiber.New()
	app.Use(v.FiberMiddleware())
	app.Get("/ok", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"sub":      c.Locals("sub"),
			"username": c.Locals("username"),
			"scope":    c.Locals("scope"),
			"groups":   c.Locals("groups"),
		})
	})
	return app
}

func TestNewCognitoVerifier_Validation(t *testing.T) {
	_, err := NewCognitoVerifier(CognitoConfig{})
	assert.Error(t, err)
	assert.Equal(t, "Region is required", err.Error())

	_, err = NewCognitoVerifier(CognitoConfig{Region: "us-east-1"})
	assert.Error(t, err)
	assert.Equal(t, "UserPoolID is required", err.Error())

	_, err = NewCognitoVerifier(CognitoConfig{Region: "us-east-1", UserPoolID: "pool"})
	assert.Error(t, err)
	assert.Equal(t, "ClientID is required", err.Error())
}

func TestNewCognitoVerifier_BuildsIssuerAndJWKSURL(t *testing.T) {
	v, err := NewCognitoVerifier(CognitoConfig{
		Region:     "us-east-1",
		UserPoolID: "us-east-1_ABC123",
		ClientID:   "client-123",
	})
	require.NoError(t, err)
	require.NotNil(t, v)

	assert.Equal(t, "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_ABC123", v.issuer)
	assert.Equal(t, "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_ABC123/.well-known/jwks.json", v.jwksURL)
	assert.NotNil(t, v.cache)
	assert.NotNil(t, v.client)
	assert.Equal(t, "client-123", v.cfg.ClientID)
}

func TestNewCognitoVerifierWithURLs_Validation(t *testing.T) {
	_, err := NewCognitoVerifierWithURLs(CognitoConfig{}, "iss", "jwks")
	assert.Error(t, err)
	assert.Equal(t, "ClientID is required", err.Error())

	_, err = NewCognitoVerifierWithURLs(CognitoConfig{ClientID: "cid"}, "", "jwks")
	assert.Error(t, err)
	assert.Equal(t, "issuer is required", err.Error())

	_, err = NewCognitoVerifierWithURLs(CognitoConfig{ClientID: "cid"}, "iss", "")
	assert.Error(t, err)
	assert.Equal(t, "jwksURL is required", err.Error())
}

func TestFiberMiddleware_MissingHeader_Unauthorized(t *testing.T) {
	v, err := NewCognitoVerifierWithURLs(CognitoConfig{ClientID: "cid"}, "issuer", "jwks")
	require.NoError(t, err)

	app := makeAppWithMiddleware(v)

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestFiberMiddleware_JWKSFetchFailure_UnauthorizedWithMessage(t *testing.T) {
	jwksSrv := newFailingJWKSserver()
	defer jwksSrv.Close()

	v, err := NewCognitoVerifierWithURLs(
		CognitoConfig{ClientID: "cid"},
		"test-issuer",
		jwksSrv.URL,
	)
	require.NoError(t, err)

	app := makeAppWithMiddleware(v)

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("x-amzn-oidc-accesstoken", "not-a-jwt")

	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestFiberMiddleware_InvalidToken_Unauthorized(t *testing.T) {
	priv, pub := newRSAKeyPair(t)

	set := jwk.NewSet()
	set.AddKey(pub)

	jwksSrv := newJWKSserver(t, set)
	defer jwksSrv.Close()

	issuer := "https://issuer.example"
	clientID := "client-123"

	v, err := NewCognitoVerifierWithURLs(CognitoConfig{ClientID: clientID}, issuer, jwksSrv.URL)
	require.NoError(t, err)

	app := makeAppWithMiddleware(v)

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("x-amzn-oidc-accesstoken", "definitely-not-a-jwt")

	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	_ = priv
}

func TestFiberMiddleware_WrongIssuer_Unauthorized(t *testing.T) {
	priv, pub := newRSAKeyPair(t)

	set := jwk.NewSet()
	set.AddKey(pub)

	jwksSrv := newJWKSserver(t, set)
	defer jwksSrv.Close()

	issuer := "https://issuer.example"
	clientID := "client-123"

	v, err := NewCognitoVerifierWithURLs(CognitoConfig{ClientID: clientID}, issuer, jwksSrv.URL)
	require.NoError(t, err)

	badToken := signAccessToken(t, priv, "https://different-issuer.example", clientID, nil)

	app := makeAppWithMiddleware(v)

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("x-amzn-oidc-accesstoken", badToken)

	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestFiberMiddleware_WrongTokenUse_Unauthorized(t *testing.T) {
	priv, pub := newRSAKeyPair(t)

	set := jwk.NewSet()
	set.AddKey(pub)

	jwksSrv := newJWKSserver(t, set)
	defer jwksSrv.Close()

	issuer := "https://issuer.example"
	clientID := "client-123"

	v, err := NewCognitoVerifierWithURLs(CognitoConfig{ClientID: clientID}, issuer, jwksSrv.URL)
	require.NoError(t, err)

	tok := jwt.New()
	require.NoError(t, tok.Set(jwt.IssuerKey, issuer))
	require.NoError(t, tok.Set("token_use", "id")) // wrong
	require.NoError(t, tok.Set("client_id", clientID))

	hdrs := jws.NewHeaders()
	require.NoError(t, hdrs.Set(jws.KeyIDKey, "test-kid"))

	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256, priv, jws.WithProtectedHeaders(hdrs)))
	require.NoError(t, err)

	app := makeAppWithMiddleware(v)

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("x-amzn-oidc-accesstoken", string(signed))

	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestFiberMiddleware_WrongClientID_Unauthorized(t *testing.T) {
	priv, pub := newRSAKeyPair(t)

	set := jwk.NewSet()
	set.AddKey(pub)

	jwksSrv := newJWKSserver(t, set)
	defer jwksSrv.Close()

	issuer := "https://issuer.example"

	v, err := NewCognitoVerifierWithURLs(CognitoConfig{ClientID: "expected-client"}, issuer, jwksSrv.URL)
	require.NoError(t, err)

	badToken := signAccessToken(t, priv, issuer, "other-client", nil)

	app := makeAppWithMiddleware(v)

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("x-amzn-oidc-accesstoken", badToken)

	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestFiberMiddleware_ValidToken_SetsLocals_AllowsRequest(t *testing.T) {
	priv, pub := newRSAKeyPair(t)

	set := jwk.NewSet()
	set.AddKey(pub)

	jwksSrv := newJWKSserver(t, set)
	defer jwksSrv.Close()

	issuer := "https://issuer.example"
	clientID := "client-123"

	v, err := NewCognitoVerifierWithURLs(CognitoConfig{ClientID: clientID}, issuer, jwksSrv.URL)
	require.NoError(t, err)

	token := signAccessToken(t, priv, issuer, clientID, nil)

	app := makeAppWithMiddleware(v)

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("x-amzn-oidc-accesstoken", token)

	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	var got map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))

	assert.Equal(t, "user-123", got["sub"])
	assert.Equal(t, "imhotep", got["username"])
	assert.Equal(t, "read:all", got["scope"])

	groups, ok := got["groups"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, groups)
	assert.Equal(t, "admins", groups[0])
}
