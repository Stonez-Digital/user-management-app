package database

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestPostgres17PgcryptoHMACUsesDiscoveredExtensionSchema validates the exact
// pgcrypto primitive required by the signed-context verifier. It installs the
// extension in a disposable schema only when the test database does not
// already have it, then rolls the entire probe back.
func TestPostgres17PgcryptoHMACUsesDiscoveredExtensionSchema(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect disposable PostgreSQL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin disposable extension probe: %v", tx.Error)
	}
	defer tx.Rollback()

	var extensionSchema string
	if err := tx.Raw("SELECT namespace.nspname FROM pg_extension AS extension JOIN pg_namespace AS namespace ON namespace.oid = extension.extnamespace WHERE extension.extname = 'pgcrypto'").Scan(&extensionSchema).Error; err != nil {
		t.Fatalf("inspect pgcrypto extension schema: %v", err)
	}

	if extensionSchema == "" {
		extensionSchema = "pgcrypto_probe_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		quotedSchema := "\"" + strings.ReplaceAll(extensionSchema, "\"", "\"\"") + "\""
		if err := tx.Exec("CREATE SCHEMA " + quotedSchema).Error; err != nil {
			t.Fatalf("create disposable pgcrypto schema: %v", err)
		}
		if err := tx.Exec("CREATE EXTENSION pgcrypto WITH SCHEMA " + quotedSchema).Error; err != nil {
			t.Fatalf("install pgcrypto into disposable schema: %v", err)
		}
	}

	quotedSchema := "\"" + strings.ReplaceAll(extensionSchema, "\"", "\"\"") + "\""
	var actual string
	query := "SELECT encode(" + quotedSchema + ".hmac(convert_to('stonez-context-probe', 'UTF8'), convert_to('disposable-key', 'UTF8'), 'sha256'), 'hex')"
	if err := tx.Raw(query).Scan(&actual).Error; err != nil {
		t.Fatalf("execute schema-qualified pgcrypto.hmac in %q: %v", extensionSchema, err)
	}

	mac := hmac.New(sha256.New, []byte("disposable-key"))
	_, _ = mac.Write([]byte("stonez-context-probe"))
	expected := hex.EncodeToString(mac.Sum(nil))
	if actual != expected {
		t.Fatalf("pgcrypto HMAC mismatch in schema %q: got %s, want %s", extensionSchema, actual, expected)
	}
	t.Logf("pgcrypto HMAC verified through discovered extension schema %q", extensionSchema)
}
