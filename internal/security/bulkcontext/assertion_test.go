package bulkcontext

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type testKeys struct {
	active Key
	byID   map[string]Key
}

func (k testKeys) ActiveSigningKey(context.Context) (Key, error) {
	return k.active, nil
}

func (k testKeys) VerificationKey(_ context.Context, id string) (Key, error) {
	key, ok := k.byID[id]
	if !ok {
		return Key{}, errors.New("unknown key")
	}
	return key, nil
}

func testSigner(t *testing.T) (*Signer, testKeys, Binding) {
	t.Helper()
	key := Key{ID: "test-key-1", Secret: []byte("0123456789abcdef0123456789abcdef")}
	keys := testKeys{active: key, byID: map[string]Key{key.ID: key}}
	signer, err := NewSigner(keys, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	signer.now = func() time.Time { return time.Unix(1_800_000_000, 0).UTC() }
	binding := Binding{
		JobID: uuid.New(), SchoolID: uuid.New(), InitiatedBy: uuid.New(),
		WorkerID: uuid.New(), BackendPID: 1234, TransactionID: "987654321", LeaseGeneration: 7,
	}
	return signer, keys, binding
}

func TestSignAndVerifyAssertion(t *testing.T) {
	signer, keys, binding := testSigner(t)
	assertion, err := signer.Sign(context.Background(), binding, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if assertion.Claims.JobID != binding.JobID ||
		assertion.Claims.SchoolID != binding.SchoolID ||
		assertion.Claims.InitiatedBy != binding.InitiatedBy ||
		assertion.Claims.WorkerID != binding.WorkerID ||
		assertion.Claims.BackendPID != binding.BackendPID ||
		assertion.Claims.TransactionID != binding.TransactionID ||
		assertion.Claims.LeaseGeneration != binding.LeaseGeneration {
		t.Fatal("signed claims do not match trusted queue binding")
	}
	if err := VerifySignature(context.Background(), keys, assertion, time.Unix(1_800_000_030, 0), 2*time.Minute); err != nil {
		t.Fatalf("valid assertion rejected: %v", err)
	}
}

func TestSignRejectsMissingBindingAndInvalidTTL(t *testing.T) {
	signer, _, binding := testSigner(t)
	if _, err := signer.Sign(context.Background(), Binding{}, time.Minute); !errors.Is(err, ErrInvalidClaims) {
		t.Fatalf("missing binding error = %v, want ErrInvalidClaims", err)
	}
	if _, err := signer.Sign(context.Background(), binding, 3*time.Minute); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("excessive TTL error = %v, want ErrInvalidTTL", err)
	}
}

func TestSignRejectsNonCanonicalTransactionID(t *testing.T) {
	signer, _, binding := testSigner(t)
	for _, value := range []string{"", "0", "000123", "-1", "not-a-xid"} {
		binding.TransactionID = value
		if _, err := signer.Sign(context.Background(), binding, time.Minute); !errors.Is(err, ErrInvalidClaims) {
			t.Fatalf("transaction ID %q error = %v, want ErrInvalidClaims", value, err)
		}
	}
}

func TestSignRejectsSubsecondTTL(t *testing.T) {
	signer, _, binding := testSigner(t)
	if _, err := signer.Sign(context.Background(), binding, 500*time.Millisecond); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("subsecond TTL error = %v, want ErrInvalidTTL", err)
	}
}

func TestVerifyRejectsModifiedSignature(t *testing.T) {
	signer, keys, binding := testSigner(t)
	assertion, err := signer.Sign(context.Background(), binding, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if assertion.Signature[0] == 'A' {
		assertion.Signature = "B" + assertion.Signature[1:]
	} else {
		assertion.Signature = "A" + assertion.Signature[1:]
	}
	if err := VerifySignature(context.Background(), keys, assertion, time.Unix(1_800_000_030, 0), 2*time.Minute); !errors.Is(err, ErrInvalidMAC) {
		t.Fatalf("modified signature error = %v, want ErrInvalidMAC", err)
	}
}

func TestVerifyRejectsTamperedClaims(t *testing.T) {
	signer, keys, binding := testSigner(t)
	assertion, err := signer.Sign(context.Background(), binding, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	assertion.Claims.SchoolID = uuid.New()
	if err := VerifySignature(context.Background(), keys, assertion, time.Unix(1_800_000_030, 0), 2*time.Minute); !errors.Is(err, ErrInvalidMAC) {
		t.Fatalf("tampered claim error = %v, want ErrInvalidMAC", err)
	}
}

func TestVerifyRejectsExpiredAssertion(t *testing.T) {
	signer, keys, binding := testSigner(t)
	assertion, err := signer.Sign(context.Background(), binding, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(context.Background(), keys, assertion, time.Unix(1_800_000_061, 0), 2*time.Minute); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("expired assertion error = %v, want ErrInvalidTTL", err)
	}
}

func TestVerifyRejectsReplaySignatureForAlteredAssertionID(t *testing.T) {
	signer, keys, binding := testSigner(t)
	assertion, err := signer.Sign(context.Background(), binding, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	assertion.Claims.AssertionID = uuid.New()
	if err := VerifySignature(context.Background(), keys, assertion, time.Unix(1_800_000_030, 0), 2*time.Minute); !errors.Is(err, ErrInvalidMAC) {
		t.Fatalf("altered assertion ID error = %v, want ErrInvalidMAC", err)
	}
}

func TestVerifyRejectsUnknownKeyID(t *testing.T) {
	signer, keys, binding := testSigner(t)
	assertion, err := signer.Sign(context.Background(), binding, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	assertion.Claims.KeyID = "retired-or-unknown"
	if err := VerifySignature(context.Background(), keys, assertion, time.Unix(1_800_000_030, 0), 2*time.Minute); err == nil {
		t.Fatal("unknown key ID was accepted")
	}
}

func TestCanonicalPayloadIsStableAndFieldOrdered(t *testing.T) {
	claims := Claims{
		KeyID: "key-1", AssertionID: uuid.MustParse("00000000-0000-4000-8000-000000000001"),
		JobID: uuid.MustParse("00000000-0000-4000-8000-000000000002"),
		SchoolID: uuid.MustParse("00000000-0000-4000-8000-000000000003"),
		InitiatedBy: uuid.MustParse("00000000-0000-4000-8000-000000000004"),
		WorkerID: uuid.MustParse("00000000-0000-4000-8000-000000000005"),
		BackendPID: 1234, TransactionID: 987654321,
		LeaseGeneration: 9, IssuedAt: 1800000000, ExpiresAt: 1800000060,
	}
	got, err := CanonicalPayload(claims)
	if err != nil {
		t.Fatal(err)
	}
	want := "stonez-bulk-import-context-v1\nkey-1\n00000000-0000-4000-8000-000000000001\n00000000-0000-4000-8000-000000000002\n00000000-0000-4000-8000-000000000003\n00000000-0000-4000-8000-000000000004\n00000000-0000-4000-8000-000000000005\n1234\n987654321\n9\n1800000000\n1800000060"
	if string(got) != want {
		t.Fatalf("canonical payload mismatch\n got: %q\nwant: %q", got, want)
	}
}
