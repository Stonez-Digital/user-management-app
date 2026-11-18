package bulkimportcontext

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	// Issuer and audience are intentionally fixed. They are part of the
	// verifier contract and must not be supplied by a request or environment.
	AssertionIssuer   = "stonez-bulk-import-worker"
	AssertionAudience = "stonez-postgres-bulk-import-context"
	DefaultTTL        = 30 * time.Second
	MaxTTL            = 60 * time.Second
	minimumKeyBytes   = 32
)

// Context identifies the exact queue lease for which a database transaction
// is being authorized. All fields must come from the restricted claim result.
type Context struct {
	JobID       uuid.UUID
	SchoolID    uuid.UUID
	InitiatedBy uuid.UUID
	WorkerID    uuid.UUID
	LeaseID     uuid.UUID
}

// Claims is the signed assertion payload. RegisteredClaims provide the
// standard issuer, audience, subject, token ID and time claims.
type Claims struct {
	JobID       string `json:"job_id"`
	SchoolID    string `json:"school_id"`
	InitiatedBy string `json:"initiated_by"`
	WorkerID    string `json:"worker_id"`
	LeaseID     string `json:"lease_id"`
	jwt.RegisteredClaims
}

// Signer issues short-lived HS256 assertions. The key must be independently
// provisioned into the trusted worker and the private PostgreSQL verifier key
// store; it must never be stored in a database GUC or job record.
type Signer struct {
	keyID string
	key   []byte
	ttl   time.Duration
	now   func() time.Time
}

// NewSigner constructs a signer from raw key bytes. Keys shorter than 256 bits
// are rejected. The caller retains responsibility for secret custody.
func NewSigner(keyID string, key []byte, ttl time.Duration) (*Signer, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" || len(keyID) > 64 {
		return nil, errors.New("bulk import assertion key ID must contain 1 to 64 characters")
	}
	if strings.ContainsAny(keyID, " \t\r\n") {
		return nil, errors.New("bulk import assertion key ID must not contain whitespace")
	}
	if len(key) < minimumKeyBytes {
		return nil, fmt.Errorf("bulk import assertion key must be at least %d bytes", minimumKeyBytes)
	}
	if ttl <= 0 || ttl > MaxTTL {
		return nil, fmt.Errorf("bulk import assertion TTL must be greater than zero and no longer than %s", MaxTTL)
	}

	keyCopy := append([]byte(nil), key...)
	return &Signer{keyID: keyID, key: keyCopy, ttl: ttl, now: time.Now}, nil
}

// NewSignerFromBase64URL decodes an unpadded base64url secret from a secret
// manager/environment variable and applies the same minimum key-size checks.
func NewSignerFromBase64URL(keyID, encodedKey string, ttl time.Duration) (*Signer, error) {
	encodedKey = strings.TrimSpace(encodedKey)
	if encodedKey == "" {
		return nil, errors.New("bulk import assertion signing key is required")
	}
	key, err := base64.RawURLEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("decode bulk import assertion signing key: %w", err)
	}
	return NewSigner(keyID, key, ttl)
}

// Sign creates one assertion for one database transaction. Callers must mint
// a fresh assertion (and therefore a fresh jti) for every transaction.
func (s *Signer) Sign(ctx Context) (string, error) {
	if s == nil || len(s.key) < minimumKeyBytes {
		return "", errors.New("bulk import assertion signer is not configured")
	}
	if ctx.JobID == uuid.Nil || ctx.SchoolID == uuid.Nil || ctx.InitiatedBy == uuid.Nil ||
		ctx.WorkerID == uuid.Nil || ctx.LeaseID == uuid.Nil {
		return "", errors.New("bulk import assertion requires job, school, initiator, worker and lease IDs")
	}

	now := s.now().UTC()
	claims := Claims{
		JobID:       ctx.JobID.String(),
		SchoolID:    ctx.SchoolID.String(),
		InitiatedBy: ctx.InitiatedBy.String(),
		WorkerID:    ctx.WorkerID.String(),
		LeaseID:     ctx.LeaseID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    AssertionIssuer,
			Subject:   ctx.WorkerID.String(),
			Audience:  jwt.ClaimStrings{AssertionAudience},
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = s.keyID
	signed, err := token.SignedString(s.key)
	if err != nil {
		return "", fmt.Errorf("sign bulk import context assertion: %w", err)
	}
	return signed, nil
}
