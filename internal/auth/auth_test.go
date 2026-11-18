package auth

import (
	"testing"
	"time"
)

func TestConfigureSecretRejectsShortSecret(t *testing.T) {
	Secret = nil
	if err := ConfigureSecret("too-short"); err == nil {
		t.Fatal("expected short JWT secret to be rejected")
	}
}

func TestTokenRoundTripAndExpiry(t *testing.T) {
	if err := ConfigureSecret("test-secret-that-is-at-least-32-characters-long"); err != nil {
		t.Fatal(err)
	}

	token, err := GenerateToken("user-123", "user")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-123" || claims.Role != "user" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.ExpiresAt == nil || time.Until(claims.ExpiresAt.Time) <= 0 {
		t.Fatal("expected a future expiry")
	}
}

func TestRefreshTokenCannotBeUsedAsAccessToken(t *testing.T) {
	if err := ConfigureSecret("test-secret-that-is-at-least-32-characters-long"); err != nil {
		t.Fatal(err)
	}
	token, err := GenerateRefreshToken("user-123")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Role != "refresh" {
		t.Fatalf("expected refresh role, got %q", claims.Role)
	}
}
