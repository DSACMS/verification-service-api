package veterans

import (
	"crypto/rsa"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// BuildClientAssertion builds and signs the JWT used as client_assertion.
//
// clientID: the VA-issued client id \n
//
// privateKeyPath: path to private.pem
//
// audience: the Okta token recipient URL (the "aud" from VA docs table)
//
// kid: optional key id ("" if not used)
func BuildClientAssertion(clientID, privateKeyPath, audience string) (string, error) {
	if clientID == "" || privateKeyPath == "" || audience == "" {
		return "", errors.New("clientID, privateKeyPath, and audience are required")
	}

	now := time.Now().UTC()
	iat := now.Unix()

	// From VA API DOCS:
	//
	// Integer. A timestamp for when the token will expire, given in seconds since January 1, 1970. This claim fails the request if the expiration time is more than 300 seconds (5 minutes) after the iat.
	var exp int64 = now.Add(4 * time.Minute).Unix()

	keyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", err
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return "", err
	}

	claims := jwt.MapClaims{
		"aud": audience,
		"iss": clientID,
		"sub": clientID,
		"iat": iat,
		"exp": exp,
		"jti": uuid.NewString(), // string
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)



	signed, err := tok.SignedString(privateKey)
	if err != nil {
		return "", err
	}
	return signed, nil
}

// If you prefer explicit rsa parsing (same thing under the hood):
func parseRSAPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	return jwt.ParseRSAPrivateKeyFromPEM(pemBytes)
}
