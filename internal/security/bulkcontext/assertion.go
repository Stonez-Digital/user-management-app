package bulkcontext

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const assertionVersion = "stonez-bulk-import-context-v1"
const minimumKeyBytes = 32

var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

var (
	ErrInvalidClaims = errors.New("invalid bulk import context claims")
	ErrInvalidKey    = errors.New("invalid bulk import context signing key")
	ErrInvalidTTL    = errors.New("invalid bulk import context assertion TTL")
	ErrInvalidMAC    = errors.New("invalid bulk import context signature")
)

// Binding contains values obtained from the trusted queue-claim result. It must
// never be populated from spreadsheet rows or other user-controlled payloads.
type Binding struct {
	JobID           uuid.UUID
	SchoolID        uuid.UUID
	InitiatedBy     uuid.UUID
	WorkerID        uuid.UUID
	BackendPID      int32
	TransactionID   string
	LeaseGeneration int64
}

// Claims are the signed, short-lived identity and lease claims. Timestamps are
// Unix seconds so Go and PostgreSQL can reconstruct the exact same payload.
type Claims struct {
	KeyID           string    `json:"kid"`
	AssertionID     uuid.UUID `json:"jti"`
	JobID           uuid.UUID `json:"job_id"`
	SchoolID        uuid.UUID `json:"school_id"`
	InitiatedBy     uuid.UUID `json:"initiated_by"`
	WorkerID        uuid.UUID `json:"worker_id"`
	BackendPID      int32     `json:"backend_pid"`
	TransactionID   string    `json:"transaction_id"`
	LeaseGeneration int64    `json:"lease_generation"`
	IssuedAt        int64     `json:"iat"`
	ExpiresAt       int64     `json:"exp"`
}

// Assertion carries claims and a raw-URL-base64 encoded HMAC-SHA256.
type Assertion struct {
	Claims    Claims `json:"claims"`
	Signature string `json:"signature"`
}

// Key is supplied by a secret manager adapter. Secret bytes must never be
// logged, persisted in source control, or returned to a database caller.
type Key struct {
	ID     string
	Secret []byte
}

// SigningKeySource exposes only the active signing key to the worker.
// VerificationKeySource is reserved for local verification tests/tools; the
// production worker must not receive retiring verifier keys.
type SigningKeySource interface {
	ActiveSigningKey(context.Context) (Key, error)
}

type VerificationKeySource interface {
	VerificationKey(context.Context, string) (Key, error)
}

type KeySource interface {
	SigningKeySource
	VerificationKeySource
}

// Signer creates assertions from trusted claim results.
type Signer struct {
	keys   SigningKeySource
	now    func() time.Time
	maxTTL time.Duration
}

func NewSigner(keys SigningKeySource, maxTTL time.Duration) (*Signer, error) {
	if keys == nil || maxTTL < time.Second || maxTTL%time.Second != 0 {
		return nil, ErrInvalidTTL
	}
	return &Signer{keys: keys, now: time.Now, maxTTL: maxTTL}, nil
}

// Sign creates a unique assertion with a bounded lifetime.
func (s *Signer) Sign(ctx context.Context, binding Binding, ttl time.Duration) (Assertion, error) {
	if s == nil || s.keys == nil || ttl < time.Second || ttl%time.Second != 0 || ttl > s.maxTTL {
		return Assertion{}, ErrInvalidTTL
	}
	if binding.JobID == uuid.Nil || binding.SchoolID == uuid.Nil ||
		binding.InitiatedBy == uuid.Nil || binding.WorkerID == uuid.Nil ||
		binding.BackendPID <= 0 || !validTransactionID(binding.TransactionID) || binding.LeaseGeneration <= 0 {
		return Assertion{}, ErrInvalidClaims
	}

	key, err := s.keys.ActiveSigningKey(ctx)
	if err != nil {
		return Assertion{}, fmt.Errorf("load active bulk import signing key: %w", err)
	}
	key.Secret = append([]byte(nil), key.Secret...)
	if err := validateKey(key); err != nil {
		zeroSecret(key.Secret)
		return Assertion{}, err
	}
	defer zeroSecret(key.Secret)

	now := s.now().UTC().Truncate(time.Second)
	claims := Claims{
		KeyID: key.ID, AssertionID: uuid.New(),
		JobID: binding.JobID, SchoolID: binding.SchoolID,
		InitiatedBy: binding.InitiatedBy, WorkerID: binding.WorkerID, BackendPID: binding.BackendPID,
		TransactionID: binding.TransactionID, LeaseGeneration: binding.LeaseGeneration,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(ttl).Unix(),
	}
	payload, err := CanonicalPayload(claims)
	if err != nil {
		return Assertion{}, err
	}
	mac := hmac.New(sha256.New, key.Secret)
	_, _ = mac.Write(payload)
	return Assertion{
		Claims: claims,
		Signature: base64.RawURLEncoding.EncodeToString(mac.Sum(nil)),
	}, nil
}

// CanonicalPayload defines the cross-language signing contract. PostgreSQL
// must reconstruct these exact UTF-8 bytes in this exact field order.
func CanonicalPayload(c Claims) ([]byte, error) {
	if err := validateClaimsShape(c); err != nil {
		return nil, err
	}
	fields := []string{
		assertionVersion,
		c.KeyID,
		c.AssertionID.String(),
		c.JobID.String(),
		c.SchoolID.String(),
		c.InitiatedBy.String(),
		c.WorkerID.String(),
		strconv.FormatInt(int64(c.BackendPID), 10),
		c.TransactionID,
		strconv.FormatInt(c.LeaseGeneration, 10),
		strconv.FormatInt(c.IssuedAt, 10),
		strconv.FormatInt(c.ExpiresAt, 10),
	}
	return []byte(strings.Join(fields, "\n")), nil
}

// VerifySignature is intended for unit tests and diagnostics. Runtime
// authorization must be performed by the PostgreSQL verifier in the same
// transaction as the tenant operation.
func VerifySignature(ctx context.Context, keys VerificationKeySource, assertion Assertion, now time.Time, maxTTL time.Duration) error {
	if keys == nil {
		return ErrInvalidKey
	}
	if err := validateClaimsShape(assertion.Claims); err != nil {
		return err
	}
	current := now.UTC().Unix()
	maxTTLSeconds := int64(maxTTL / time.Second)
	if maxTTLSeconds < 1 || assertion.Claims.IssuedAt > current+30 ||
		assertion.Claims.ExpiresAt <= current ||
		assertion.Claims.ExpiresAt <= assertion.Claims.IssuedAt ||
		assertion.Claims.ExpiresAt-assertion.Claims.IssuedAt > maxTTLSeconds {
		return ErrInvalidTTL
	}
	provided, err := base64.RawURLEncoding.DecodeString(assertion.Signature)
	if err != nil || len(provided) != sha256.Size {
		return ErrInvalidMAC
	}
	key, err := keys.VerificationKey(ctx, assertion.Claims.KeyID)
	if err != nil {
		return fmt.Errorf("load bulk import verification key: %w", err)
	}
	key.Secret = append([]byte(nil), key.Secret...)
	if err := validateKey(key); err != nil || key.ID != assertion.Claims.KeyID {
		zeroSecret(key.Secret)
		return ErrInvalidKey
	}
	defer zeroSecret(key.Secret)
	payload, err := CanonicalPayload(assertion.Claims)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, key.Secret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return ErrInvalidMAC
	}
	return nil
}

func validateKey(key Key) error {
	if !keyIDPattern.MatchString(key.ID) || len(key.Secret) < minimumKeyBytes {
		return ErrInvalidKey
	}
	return nil
}

func validateClaimsShape(c Claims) error {
	if !keyIDPattern.MatchString(c.KeyID) ||
		c.AssertionID == uuid.Nil || c.JobID == uuid.Nil ||
		c.SchoolID == uuid.Nil || c.InitiatedBy == uuid.Nil ||
		c.WorkerID == uuid.Nil || c.BackendPID <= 0 || !validTransactionID(c.TransactionID) || c.LeaseGeneration <= 0 ||
		c.IssuedAt <= 0 || c.ExpiresAt <= c.IssuedAt {
		return ErrInvalidClaims
	}
	return nil
}

func validTransactionID(value string) bool {
	parsed, err := strconv.ParseUint(value, 10, 64)
	return err == nil && parsed > 0 && strconv.FormatUint(parsed, 10) == value
}
