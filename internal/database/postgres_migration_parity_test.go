package database

import (
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestPostgresMigrationCatalogBaseline runs the real Go migration runner in a
// unique disposable schema. It deliberately does not create RLS policies or
// grants in the test schema: the catalog is the evidence under inspection.
//
// This is a baseline/parity test, NOT proof of tenant isolation. The current
// application migration set is expected to expose the documented policy gaps
// until a separately reviewed policy migration is implemented.
func TestPostgresMigrationCatalogBaseline(t *testing.T) {
	rawDSN := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if rawDSN == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("TEST_DATABASE_URL is required in CI for PostgreSQL migration parity")
		}
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	parsed, err := url.Parse(rawDSN)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" && host != "postgres" {
		t.Fatalf("refusing migration-parity test against non-local PostgreSQL host %q; use a disposable local PostgreSQL 17 service", host)
	}

	admin, err := gorm.Open(postgres.Open(rawDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect disposable PostgreSQL service: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adminSQL.Close() })

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	schema := "migration_parity_" + suffix
	runtimeRole := "migration_parity_runtime_" + suffix
	quoteIdent := func(value string) string {
		return "\"" + strings.ReplaceAll(value, "\"", "\"\"") + "\""
	}

	if err := admin.Exec("CREATE SCHEMA " + quoteIdent(schema)).Error; err != nil {
		t.Fatalf("create disposable schema: %v", err)
	}
	roleCreated := false
	t.Cleanup(func() {
		// Drop only objects created by this test. Never touch public or shared
		// application schemas, and remove the temporary role after its schema.
		_ = admin.Exec("DROP SCHEMA IF EXISTS " + quoteIdent(schema) + " CASCADE").Error
		if roleCreated {
			_ = admin.Exec("DROP ROLE IF EXISTS " + quoteIdent(runtimeRole)).Error
		}
	})
	if err := admin.Exec("CREATE ROLE " + quoteIdent(runtimeRole) + " LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION").Error; err != nil {
		t.Fatalf("create disposable restricted runtime role: %v", err)
	}
	roleCreated = true

	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	migrationDB, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect isolated migration schema: %v", err)
	}
	migrationSQL, err := migrationDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	migrationSQL.SetMaxOpenConns(1)
	migrationSQL.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = migrationSQL.Close() })

	// Prove the connection is pinned to the unique test schema before any
	// migration executes. This prevents accidental writes to public if the
	// driver ignores or rejects the search_path connection parameter.
	var connectedSchema string
	if err := migrationDB.Raw("SELECT current_schema()").Scan(&connectedSchema).Error; err != nil {
		t.Fatalf("inspect isolated connection schema: %v", err)
	}
	if connectedSchema != schema {
		t.Fatalf("refusing to run migrations outside disposable schema: current_schema=%q expected=%q", connectedSchema, schema)
	}

	if err := Migrate(migrationDB); err != nil {
		t.Fatalf("apply actual versioned database.Migrate path in disposable schema: %v", err)
	}
	if err := Migrate(migrationDB); err != nil {
		t.Fatalf("re-run actual database.Migrate path (idempotence): %v", err)
	}

	var migrationCount int64
	if err := migrationDB.Model(&Migration{}).Count(&migrationCount).Error; err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	if migrationCount != 47 {
		t.Fatalf("expected 47 versioned Go migrations, got %d", migrationCount)
	}

	// This list is the automatic Go migration catalog, not the separate
	// review-only SQL migrations or the Supabase migration directory.
	expectedTables := []string{
		"academic_sessions", "announcements", "assessment_results", "assessments",
		"attendance_records", "audit_logs", "blog_categories", "blog_posts",
		"bulk_import_jobs", "career_path_attempts", "class_subjects", "fee_items",
		"gallery_albums", "gallery_images", "guardian_relationships", "invoice_lines",
		"invoices", "migrations", "national_curricula", "national_curriculum_classes",
		"national_curriculum_levels", "national_curriculum_subjects", "notifications",
		"parent_profiles", "password_reset_tokens", "payments", "refresh_tokens",
		"role_change_audits", "school_advertisements", "school_classes", "school_enquiries",
		"schools", "sections", "sessions", "student_enrollments", "students", "subjects",
		"teacher_assignments", "teacher_profiles", "terms", "timetable_entries", "users",
	}
	expected := make(map[string]bool, len(expectedTables))
	for _, table := range expectedTables {
		expected[table] = true
	}
	var actualTables []string
	if err := migrationDB.Raw(`
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = current_schema()
		ORDER BY tablename
	`).Scan(&actualTables).Error; err != nil {
		t.Fatalf("inspect migrated table catalog: %v", err)
	}
	actual := make(map[string]bool, len(actualTables))
	for _, table := range actualTables {
		actual[table] = true
	}
	var missing, unexpected []string
	for table := range expected {
		if !actual[table] {
			missing = append(missing, table)
		}
	}
	for _, table := range actualTables {
		if !expected[table] {
			unexpected = append(unexpected, table)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	if len(missing) != 0 || len(unexpected) != 0 {
		t.Fatalf("automatic migration table inventory mismatch: missing=%v unexpected=%v", missing, unexpected)
	}

	// These are the tables explicitly ENABLEd by the automatic Go migration
	// path. FORCE RLS and tenant policies are not currently installed by it.
	rlsEnabledTables := []string{
		"blog_categories", "blog_posts", "bulk_import_jobs", "class_subjects",
		"gallery_albums", "gallery_images", "national_curricula",
		"national_curriculum_classes", "national_curriculum_levels",
		"national_curriculum_subjects", "parent_profiles", "school_advertisements",
		"school_enquiries", "schools", "teacher_profiles",
	}
	rlsEnabled := make(map[string]bool, len(rlsEnabledTables))
	for _, table := range rlsEnabledTables {
		rlsEnabled[table] = true
	}
	for _, table := range expectedTables {
		var enabled, forced bool
		if err := migrationDB.Raw(`
			SELECT relrowsecurity, relforcerowsecurity
			FROM pg_class
			WHERE oid = to_regclass(?)
		`, schema+"."+table).Row().Scan(&enabled, &forced); err != nil {
			t.Fatalf("inspect RLS flags for %s: %v", table, err)
		}
		if enabled != rlsEnabled[table] {
			t.Errorf("%s RLS enabled=%t, expected current migration baseline=%t", table, enabled, rlsEnabled[table])
		}
		if forced {
			t.Errorf("%s unexpectedly has FORCE ROW LEVEL SECURITY in the current Go migration baseline", table)
		}
	}

	type policyRow struct {
		TableName string
		PolicyName string
		Cmd string
		Qual *string
		WithCheck *string
	}
	var policies []policyRow
	if err := migrationDB.Raw(`
		SELECT tablename AS table_name, policyname AS policy_name, cmd,
		       qual, with_check
		FROM pg_policies
		WHERE schemaname = current_schema()
		ORDER BY tablename, policyname
	`).Scan(&policies).Error; err != nil {
		t.Fatalf("inspect actual pg_policies catalog: %v", err)
	}
	if len(policies) != 0 {
		t.Fatalf("Go migration baseline unexpectedly contains policies; review and update the explicit expected policy inventory: %+v", policies)
	}
	t.Log("SECURITY GAP (expected baseline): database.Migrate installed no tenant-aware policies; this test does not prove tenant isolation.")
	t.Log("SECURITY GAP (expected baseline): database.Migrate did not FORCE RLS on the reviewed tenant-owned tables.")

	var role struct {
		CanLogin bool
		Superuser bool
		BypassRLS bool
		CreateDB bool
		CreateRole bool
	}
	if err := migrationDB.Raw(`
		SELECT rolcanlogin AS can_login, rolsuper AS superuser,
		       rolbypassrls AS bypass_rls, rolcreatedb AS create_db,
		       rolcreaterole AS create_role
		FROM pg_roles WHERE rolname = ?
	`, runtimeRole).Scan(&role).Error; err != nil {
		t.Fatalf("inspect restricted role attributes: %v", err)
	}
	if !role.CanLogin || role.Superuser || role.BypassRLS || role.CreateDB || role.CreateRole {
		t.Fatalf("temporary runtime role has unsafe attributes: %+v", role)
	}
	var membershipCount int64
	if err := migrationDB.Raw(`
		SELECT COUNT(*)
		FROM pg_auth_members
		WHERE member = (SELECT oid FROM pg_roles WHERE rolname = ?)
	`, runtimeRole).Scan(&membershipCount).Error; err != nil {
		t.Fatalf("inspect runtime role memberships: %v", err)
	}
	if membershipCount != 0 {
		t.Fatalf("temporary runtime role unexpectedly has %d role memberships", membershipCount)
	}

	var migrationOwner string
	if err := migrationDB.Raw("SELECT current_user").Scan(&migrationOwner).Error; err != nil {
		t.Fatalf("inspect migration connection owner: %v", err)
	}
	for _, table := range expectedTables {
		var owner string
		if err := migrationDB.Raw(`
			SELECT pg_get_userbyid(relowner)
			FROM pg_class
			WHERE oid = to_regclass(?)
		`, schema+"."+table).Scan(&owner).Error; err != nil {
			t.Fatalf("inspect owner for %s: %v", table, err)
		}
		if owner != migrationOwner {
			t.Errorf("%s owner=%q; expected migration connection owner %q", table, owner, migrationOwner)
		}
		if owner == runtimeRole {
			t.Errorf("runtime role unexpectedly owns protected table %s", table)
		}
		var isOwnerOrMember bool
		if err := migrationDB.Raw("SELECT pg_has_role(?, ?, 'MEMBER')", runtimeRole, owner).Scan(&isOwnerOrMember).Error; err != nil {
			t.Fatalf("inspect runtime membership in owner role for %s: %v", table, err)
		}
		if isOwnerOrMember {
			t.Errorf("runtime role is a member of owner role %q for %s", owner, table)
		}
		for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			var granted bool
			if err := migrationDB.Raw("SELECT has_table_privilege(?, ?, ?)", runtimeRole, schema+"."+table, privilege).Scan(&granted).Error; err != nil {
				t.Fatalf("inspect %s privilege on %s: %v", privilege, table, err)
			}
			if granted {
				t.Errorf("temporary runtime role unexpectedly has %s on %s", privilege, table)
			}
		}
		for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "REFERENCES"} {
			var granted bool
			if err := migrationDB.Raw("SELECT has_any_column_privilege(?, ?, ?)", runtimeRole, schema+"."+table, privilege).Scan(&granted).Error; err != nil {
				t.Fatalf("inspect column-level %s privilege on %s: %v", privilege, table, err)
			}
			if granted {
				t.Errorf("temporary runtime role unexpectedly has column-level %s on %s", privilege, table)
			}
		}
	}

	var schemaUsage, schemaCreate bool
	if err := migrationDB.Raw(`
		SELECT has_schema_privilege(?, ?, 'USAGE') AS schema_usage,
		       has_schema_privilege(?, ?, 'CREATE') AS schema_create
	`, runtimeRole, schema, runtimeRole, schema).Row().Scan(&schemaUsage, &schemaCreate); err != nil {
		t.Fatalf("inspect runtime schema privileges: %v", err)
	}
	if schemaUsage || schemaCreate {
		t.Fatalf("temporary runtime role unexpectedly has schema privileges: USAGE=%t CREATE=%t", schemaUsage, schemaCreate)
	}

	var sequencePrivileges int64
	if err := migrationDB.Raw(`
		SELECT COUNT(*)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = current_schema()
		  AND c.relkind = 'S'
		  AND (
		    has_sequence_privilege(?, c.oid, 'USAGE')
		    OR has_sequence_privilege(?, c.oid, 'SELECT')
		    OR has_sequence_privilege(?, c.oid, 'UPDATE')
		  )
	`, runtimeRole, runtimeRole, runtimeRole).Scan(&sequencePrivileges).Error; err != nil {
		t.Fatalf("inspect runtime sequence privileges: %v", err)
	}
	if sequencePrivileges != 0 {
		t.Fatalf("temporary runtime role has privileges on %d sequences", sequencePrivileges)
	}

	t.Logf("catalog baseline inspected %d application tables; RLS-enabled tables: %v; tenant policies: %d", len(expectedTables), rlsEnabledTables, len(policies))
}

