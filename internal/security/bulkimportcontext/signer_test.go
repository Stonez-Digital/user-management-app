package bulkimportcontext

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func validContext() Context {
	return Context{
		JobID:       uuid.New(),
		SchoolID:    uuid.New(),
		InitiatedBy: uuid.New(),
		WorkerID:    uuid.New(),
		LeaseID:     uuid.New(),
	}
}

func TestNewSignerRejectsWeakOrInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		keyID string
		key   []byte
		ttl   time.Duration
	}{
		{name: "empty key ID", keyID: "", key: make([]byte, 32), ttl: DefaultTTL},
		{name: "whitespace key ID", keyID: "key id", key: make([]byte, 32), ttl: DefaultTTL},
		{name: "short key", keyID: "key-1", key: make([]byte, 31), ttl: DefaultTTL},
		{name: "zero TTL", keyID: "key-1", key: make([]byte, 32), ttl: 0},
		{name: "excessive TTL", keyID: "key-1", key: make([]byte, 32), ttl: MaxTTL + time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewSigner(tt.keyID, tt.key, tt.ttl); err == nil {
				t.Fatal("expected invalid signer configuration to be rejected")
			}
		})
	}
}

func TestNewSignerFromBase64URL(t *testing.T) {
	key := make([]byte, 32)
	encoded := base64.RawURLEncoding.EncodeToString(key)
	if _, err := NewSignerFromBase64URL("key-1", encoded, DefaultTTL); err != nil {
		t.Fatalf("valid encoded key rejected: %v", err)
	}
	if _, err := NewSignerFromBase64URL("key-1", "not base64!", DefaultTTL); err == nil {
		t.Fatal("malformed encoded key accepted")
	}
}

func TestSignProducesBoundHS256Assertion(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	signer, err := NewSigner("rotation-2026-01", key, DefaultTTL)
	if err != nil {
		t.Fatal(err)
	}

	ctx := validContext()
	raw, err := signer.Sign(ctx)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}
	if strings.Count(raw, ".") != 2 {
		t.Fatalf("expected compact JWS, got %q", raw)
	}

	parsed, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			t.Fatalf("unexpected signing method: %v", token.Method)
		}
		if token.Header["kid"] != "rotation-2026-01" {
			t.Fatalf("unexpected key ID: %v", token.Header["kid"])
		}
		return key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(AssertionIssuer),
		jwt.WithAudience(AssertionAudience),
	)
	if err != nil {
		t.Fatalf("verify signed assertion: %v", err)
	}
	if !parsed.Valid {
		t.Fatal("signed assertion is invalid")
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok {
		t.Fatalf("unexpected claims type %T", parsed.Claims)
	}
	if claims.JobID != ctx.JobID.String() ||
		claims.SchoolID != ctx.SchoolID.String() ||
		claims.InitiatedBy != ctx.InitiatedBy.String() ||
		claims.WorkerID != ctx.WorkerID.String() ||
		claims.LeaseID != ctx.LeaseID.String() {
		t.Fatalf("assertion claims do not match source context: %+v", claims)
	}
	if claims.Subject != ctx.WorkerID.String() || claims.ID == "" {
		t.Fatalf("missing worker subject or unique assertion ID: %+v", claims.RegisteredClaims)
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil ||
		claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time) != DefaultTTL {
		t.Fatalf("unexpected assertion lifetime: %+v", claims.RegisteredClaims)
	}
}

func TestSignRejectsMissingContextIdentity(t *testing.T) {
	signer, err := NewSigner("key-1", []byte("0123456789abcdef0123456789abcdef"), DefaultTTL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := validContext()
	ctx.LeaseID = uuid.Nil
	if _, err := signer.Sign(ctx); err == nil {
		t.Fatal("assertion with missing lease ID was accepted")
	}
}

func TestSignUsesFreshAssertionID(t *testing.T) {
	signer, err := NewSigner("key-1", []byte("0123456789abcdef0123456789abcdef"), DefaultTTL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := validContext()
	first, err := signer.Sign(ctx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := signer.Sign(ctx)
	if err != nil {
		t.Fatal(err)
	}
	parseID := func(raw string) string {
		claims := &Claims{}
		_, parseErr := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
			return []byte("0123456789abcdef0123456789abcdef"), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if parseErr != nil {
			t.Fatalf("parse assertion: %v", parseErr)
		}
		return claims.ID
	}
	if parseID(first) == parseID(second) {
		t.Fatal("each transaction assertion must have a fresh jti")
	}
}
