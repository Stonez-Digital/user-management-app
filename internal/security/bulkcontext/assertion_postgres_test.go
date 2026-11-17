package bulkcontext

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresVerifierBindsAssertionToTransactionAndLease(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured; PostgreSQL signed-context tests require PostgreSQL 17")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil { t.Fatalf("connect disposable PostgreSQL database: %v", err) }
	adminSQL, err := admin.DB()
	if err != nil { t.Fatal(err) }
	defer adminSQL.Close()

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	tableSchema, privateSchema := "bulk_ctx_"+suffix, "bulk_ctx_private_"+suffix
	ownerRole, processRole := "bulk_ctx_owner_"+suffix, "bulk_ctx_process_"+suffix
	cleanup := func() {
		_ = admin.Exec("DROP SCHEMA IF EXISTS "+privateSchema+" CASCADE").Error
		_ = admin.Exec("DROP SCHEMA IF EXISTS "+tableSchema+" CASCADE").Error
		_ = admin.Exec("REVOKE "+ownerRole+", "+processRole+" FROM CURRENT_USER").Error
		_ = admin.Exec("DROP OWNED BY "+ownerRole).Error
		_ = admin.Exec("DROP OWNED BY "+processRole).Error
		_ = admin.Exec("DROP ROLE IF EXISTS "+ownerRole).Error
		_ = admin.Exec("DROP ROLE IF EXISTS "+processRole).Error
	}
	if err := admin.Exec("CREATE SCHEMA "+tableSchema).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec("CREATE SCHEMA "+privateSchema).Error; err != nil { cleanup(); t.Fatal(err) }
	if err := admin.Exec("CREATE SCHEMA IF NOT EXISTS extensions").Error; err != nil { cleanup(); t.Fatal(err) }
	if err := admin.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions").Error; err != nil { cleanup(); t.Fatalf("install pgcrypto in disposable test database: %v", err) }
	var hmacAvailable bool
	if err := admin.Raw("SELECT to_regprocedure('extensions.hmac(bytea,bytea,text)') IS NOT NULL").Scan(&hmacAvailable).Error; err != nil || !hmacAvailable {
		cleanup(); t.Fatalf("pgcrypto hmac is not available in extensions schema: %v", err)
	}
	if err := admin.Exec("CREATE ROLE "+ownerRole+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS").Error; err != nil { cleanup(); t.Fatal(err) }
	if err := admin.Exec("CREATE ROLE "+processRole+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS").Error; err != nil { cleanup(); t.Fatal(err) }

	createQueue := "CREATE TABLE "+tableSchema+".bulk_import_jobs ("+
		"id uuid PRIMARY KEY, school_id uuid NOT NULL, initiated_by uuid NOT NULL, "+
		"worker_id uuid, lease_generation bigint NOT NULL DEFAULT 0, status text NOT NULL, claimed_at timestamptz)"
	if err := admin.Exec(createQueue).Error; err != nil { cleanup(); t.Fatal(err) }

	scriptBytes, err := os.ReadFile("../../../scripts/security/install_bulk_import_signed_context.sql")
	if err != nil { cleanup(); t.Fatalf("read verifier script: %v", err) }
	script := strings.TrimPrefix(string(scriptBytes), "\\set ON_ERROR_STOP on\n")
	script = strings.ReplaceAll(script, "stonez_bulk_context_owner", ownerRole)
	script = strings.ReplaceAll(script, "stonez_bulk_process_group", processRole)
	script = strings.ReplaceAll(script, "stonez_internal", privateSchema)
	script = strings.ReplaceAll(script, "public", tableSchema)
	if err := admin.Exec(script).Error; err != nil { cleanup(); t.Fatalf("install verifier in isolated schema: %v", err) }
	t.Cleanup(cleanup)

	key := Key{ID:"integration-key-1", Secret:[]byte("integration-test-key-0123456789abcdef")}
	if err := admin.Exec("INSERT INTO "+privateSchema+".bulk_context_keys (key_id, secret, enabled) VALUES (?, ?, true)", key.ID, key.Secret).Error; err != nil { t.Fatal(err) }
	schoolA, schoolB, initiator, workerID, jobID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := admin.Exec("INSERT INTO "+tableSchema+".bulk_import_jobs (id, school_id, initiated_by, worker_id, lease_generation, status, claimed_at) VALUES (?, ?, ?, ?, 4, 'running', clock_timestamp())", jobID, schoolA, initiator, workerID).Error; err != nil { t.Fatal(err) }

	tenantDDL := "CREATE TABLE "+tableSchema+".tenant_fixture (id uuid PRIMARY KEY, school_id uuid NOT NULL); "+
		"ALTER TABLE "+tableSchema+".tenant_fixture ENABLE ROW LEVEL SECURITY; "+
		"ALTER TABLE "+tableSchema+".tenant_fixture FORCE ROW LEVEL SECURITY; "+
		"GRANT SELECT, INSERT ON "+tableSchema+".tenant_fixture TO "+processRole+"; "+
		"CREATE POLICY tenant_fixture_select ON "+tableSchema+".tenant_fixture FOR SELECT TO "+processRole+" "+
		"USING (EXISTS (SELECT 1 FROM "+privateSchema+".current_bulk_import_context() ctx WHERE ctx.school_id = school_id)); "+
		"CREATE POLICY tenant_fixture_insert ON "+tableSchema+".tenant_fixture FOR INSERT TO "+processRole+" "+
		"WITH CHECK (EXISTS (SELECT 1 FROM "+privateSchema+".current_bulk_import_context() ctx WHERE ctx.school_id = school_id)); "+
		"INSERT INTO "+tableSchema+".tenant_fixture (id, school_id) VALUES "+
		"('aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa', '"+schoolA.String()+"'), "+
		"('bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb', '"+schoolB.String()+"')"
	if err := admin.Exec(tenantDDL).Error; err != nil { t.Fatalf("create RLS tenant fixture: %v", err) }

	keys := testKeys{active:key, byID:map[string]Key{key.ID:key}}
	signer, err := NewSigner(keys, 2*time.Minute)
	if err != nil { t.Fatal(err) }
	signer.now = time.Now
	binding := Binding{JobID:jobID, SchoolID:schoolA, InitiatedBy:initiator, WorkerID:workerID, LeaseGeneration:4}
	assertion, err := signer.Sign(context.Background(), binding, time.Minute)
	if err != nil { t.Fatal(err) }

	verify := func(tx *gorm.DB, a Assertion) error {
		if err := tx.Exec("SET LOCAL ROLE "+processRole).Error; err != nil { return err }
		c := a.Claims
		return tx.Exec("SELECT "+privateSchema+".verify_bulk_import_context(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			c.KeyID, c.AssertionID, c.JobID, c.SchoolID, c.InitiatedBy,
			c.WorkerID, c.LeaseGeneration, c.IssuedAt, c.ExpiresAt, a.Signature).Error
	}

	rollbackMarker := errors.New("intentional rollback")
	err = admin.Transaction(func(tx *gorm.DB) error {
		if err := verify(tx, assertion); err != nil { return err }
		var visible int64
		if err := tx.Table(tableSchema+".tenant_fixture").Count(&visible).Error; err != nil { return err }
		if visible != 1 { return fmt.Errorf("school A context should see one row, got %d", visible) }
		return rollbackMarker
	})
	if !errors.Is(err, rollbackMarker) { t.Fatalf("expected intentional rollback, got %v", err) }

	// Rollback must undo assertion consumption as well as the context row.
	err = admin.Transaction(func(tx *gorm.DB) error {
		if err := verify(tx, assertion); err != nil { return err }
		var visible int64
		if err := tx.Table(tableSchema+".tenant_fixture").Count(&visible).Error; err != nil { return err }
		if visible != 1 { return fmt.Errorf("school A context should see one row, got %d", visible) }
		return tx.Exec("INSERT INTO "+tableSchema+".tenant_fixture (id, school_id) VALUES (?, ?)", uuid.New(), schoolB).Error
	})
	if err == nil { t.Fatal("verified school A context inserted a school B row") }

	// Commit a fresh assertion; it must not be reusable in another transaction.
	committed, err := signer.Sign(context.Background(), binding, time.Minute)
	if err != nil { t.Fatal(err) }
	if err := admin.Transaction(func(tx *gorm.DB) error { return verify(tx, committed) }); err != nil { t.Fatalf("commit valid assertion: %v", err) }
	if err := admin.Transaction(func(tx *gorm.DB) error { return verify(tx, committed) }); err == nil { t.Fatal("committed assertion replay was accepted") }

	// A caller-controlled GUC without a verifier-created row grants no visibility.
	err = admin.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE "+processRole).Error; err != nil { return err }
		if err := tx.Exec("SELECT set_config('stonez.bulk_context_id', ?, true)", uuid.NewString()).Error; err != nil { return err }
		var visible int64
		if err := tx.Table(tableSchema+".tenant_fixture").Count(&visible).Error; err != nil { return err }
		if visible != 0 { return fmt.Errorf("forged GUC unexpectedly exposed %d rows", visible) }
		return nil
	})
	if err != nil { t.Fatalf("forged GUC test failed: %v", err) }

	// The processing group cannot read key material directly.
	err = admin.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE "+processRole).Error; err != nil { return err }
		return tx.Raw("SELECT secret FROM "+privateSchema+".bulk_context_keys LIMIT 1").Scan(new([]byte)).Error
	})
	if err == nil { t.Fatal("processing group unexpectedly read the verifier key") }

	// A correctly signed assertion with another school still fails against the job row.
	crossSchool := binding
	crossSchool.SchoolID = schoolB
	crossSchoolAssertion, err := signer.Sign(context.Background(), crossSchool, time.Minute)
	if err != nil { t.Fatal(err) }
	if err := admin.Transaction(func(tx *gorm.DB) error { return verify(tx, crossSchoolAssertion) }); err == nil {
		t.Fatal("validly signed assertion with mismatched school was accepted")
	}

	// A replaced lease generation invalidates an assertion minted for the old generation.
	staleAssertion, err := signer.Sign(context.Background(), binding, time.Minute)
	if err != nil { t.Fatal(err) }
	if err := admin.Exec("UPDATE "+tableSchema+".bulk_import_jobs SET lease_generation = 5 WHERE id = ?", jobID).Error; err != nil { t.Fatal(err) }
	if err := admin.Transaction(func(tx *gorm.DB) error { return verify(tx, staleAssertion) }); err == nil {
		t.Fatal("assertion for replaced lease generation was accepted")
	}
}
