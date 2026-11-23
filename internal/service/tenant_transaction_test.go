package service

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTenantTransactionRejectsPartialContext(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	err = tenantTransaction(db, uuid.New(), uuid.Nil, func(*gorm.DB) error { return nil })
	if err == nil { t.Fatal("expected partial tenant context to fail closed") }
}

func TestTenantTransactionRunsCallbackForSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	called := false
	err = tenantTransaction(db, uuid.New(), uuid.New(), func(tx *gorm.DB) error {
		called = tx != nil
		return nil
	})
	if err != nil { t.Fatal(err) }
	if !called { t.Fatal("tenant transaction callback was not invoked") }
}

func TestTenantTransactionPropagatesCallbackError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	want := errors.New("abort")
	err = tenantTransaction(db, uuid.New(), uuid.New(), func(*gorm.DB) error { return want })
	if !errors.Is(err, want) { t.Fatalf("expected callback error %v, got %v", want, err) }
}
