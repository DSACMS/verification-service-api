package veterans

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestGetAssertionPrivatekey_ExpWithinFiveMinutes(t *testing.T) {
	pemPath := writeTempPEMKey(t)

	tokenString, err := GetAssertionPrivatekey("client-id", pemPath, "https://example.com/token")
	if err != nil {
		t.Fatalf("GetAssertionPrivatekey: %v", err)
	}

	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("ParseUnverified: %v", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatalf("unexpected claims type: %T", token.Claims)
	}

	iat, ok := claims["iat"].(float64)
	if !ok {
		t.Fatalf("iat claim missing or wrong type: %T", claims["iat"])
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("exp claim missing or wrong type: %T", claims["exp"])
	}

	if diff := int64(exp - iat); diff > 300 {
		t.Fatalf("expected exp-iat <= 300 seconds, got %d", diff)
	}
}
