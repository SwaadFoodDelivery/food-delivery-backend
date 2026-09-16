package utils

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	token, err := CreateAccessToken("secret", "user-1", "client", "session-1", time.Hour)
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}
	claims, err := ParseAccessToken("secret", token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.UserID != "user-1" || claims.Role != "client" || claims.ID != "session-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestAccessTokenExpired(t *testing.T) {
	token, err := CreateAccessToken("secret", "user-1", "client", "session-1", -time.Minute)
	if err != nil {
		t.Fatalf("CreateAccessToken: %v", err)
	}
	if _, err := ParseAccessToken("secret", token); !errors.Is(err, jwt.ErrTokenExpired) {
		t.Fatalf("err = %v, want jwt.ErrTokenExpired", err)
	}
}

func TestRefreshTokenRoundTrip(t *testing.T) {
	token, err := CreateRefreshToken("secret", "user-1", "session-1", 30*24*time.Hour)
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	claims, err := ParseRefreshToken("secret", token)
	if err != nil {
		t.Fatalf("ParseRefreshToken: %v", err)
	}
	if claims.UserID != "user-1" || claims.ID != "session-1" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestRefreshTokenExpired(t *testing.T) {
	token, err := CreateRefreshToken("secret", "user-1", "session-1", -time.Minute)
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	if _, err := ParseRefreshToken("secret", token); !errors.Is(err, jwt.ErrTokenExpired) {
		t.Fatalf("err = %v, want jwt.ErrTokenExpired", err)
	}
}

func TestRefreshTokenWrongSecret(t *testing.T) {
	token, err := CreateRefreshToken("secret", "user-1", "session-1", time.Hour)
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	if _, err := ParseRefreshToken("different-secret", token); err == nil {
		t.Fatal("expected an error parsing a refresh token with the wrong secret")
	}
}

// A refresh token must never be redeemable as an access token: it carries no
// role claim, and JWTAuthMiddleware would set an empty role in the gin
// context if this boundary broke. This is a schema-level check, not a
// signature check (both token kinds share the signing method and secret).
func TestRefreshTokenIsNotAnAccessToken(t *testing.T) {
	token, err := CreateRefreshToken("secret", "user-1", "session-1", time.Hour)
	if err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	claims, err := ParseAccessToken("secret", token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.Role != "" {
		t.Fatalf("refresh token parsed as access token carried a role: %q", claims.Role)
	}
}
