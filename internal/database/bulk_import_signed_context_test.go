package database

import (
	"os"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openSignedContextTestDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open signed-context test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestBulkImportSignedContextReadinessAndProcessingRole(t *testing.T) {
	runnerDSN := os.Getenv("BULK_CONTEXT_TEST_DATABASE_URL")
	workerDSN := os.Getenv("BULK_CONTEXT_TEST_WORKER_URL")
	if runnerDSN == "" || workerDSN == "" {
		t.Skip("disposable PostgreSQL signed-context test URLs are not set")
	}

	runnerDB := openSignedContextTestDB(t, runnerDSN)
	if err := ValidateBulkImportProcessingRole(runnerDB); err == nil {
		t.Fatal("privileged database owner was accepted as the processing login")
	}

	workerDB := openSignedContextTestDB(t, workerDSN)
	if err := ValidateBulkImportProcessingRole(workerDB); err != nil {
		t.Fatalf("dedicated processing login rejected: %v", err)
	}
	var catalogPolicies []struct {
		TableName string
		Name string
		Roles string
		Command string
		Permissive bool
		UsingExpr string
		CheckExpr string
		UsingNormalized string
		CheckNormalized string
	}
	if err := workerDB.Raw(`SELECT relation.relname AS table_name, policy.polname AS name,
		policy.polroles::text AS roles, policy.polcmd::text AS command,
		policy.polpermissive AS permissive,
		COALESCE(pg_get_expr(policy.polqual,policy.polrelid),'') AS using_expr,
		COALESCE(pg_get_expr(policy.polwithcheck,policy.polrelid),'') AS check_expr,
		regexp_replace(regexp_replace(COALESCE(pg_get_expr(policy.polqual,policy.polrelid),''),'::[a-zA-Z0-9_.]+','','g'),'[()[:space:]]','','g') AS using_normalized,
		regexp_replace(regexp_replace(COALESCE(pg_get_expr(policy.polwithcheck,policy.polrelid),''),'::[a-zA-Z0-9_.]+','','g'),'[()[:space:]]','','g') AS check_normalized
		FROM pg_policy AS policy JOIN pg_class AS relation ON relation.oid=policy.polrelid
		WHERE relation.oid IN ('public.bulk_import_jobs'::regclass,'public.users'::regclass,
			'public.students'::regclass,'public.teacher_profiles'::regclass,
			'public.parent_profiles'::regclass,'public.academic_sessions'::regclass,
			'public.school_classes'::regclass,'public.sections'::regclass,
			'public.student_enrollments'::regclass,'public.guardian_relationships'::regclass)
		ORDER BY relation.relname,policy.polname`).Scan(&catalogPolicies).Error; err != nil {
		t.Fatalf("inspect installed worker policy catalog: %v", err)
	}
	t.Logf("installed worker policy catalog: %+v", catalogPolicies)
	if err := ValidateBulkImportSignedContextReady(workerDB); err != nil {
		t.Fatalf("processing login cannot inspect verifier readiness: %v", err)
	}

	// Every worker policy must retain its exact role, command, permissiveness,
	// USING expression, and WITH CHECK expression. Exercise each policy by
	// changing one predicate, then restore it before the next case.
	type policyMutation struct {
		name string
		weaken string
		restore string
	}
	tenant := "school_id = stonez_context_private.current_worker_school_id()"
	queue := "school_id = stonez_context_private.current_worker_school_id() AND initiated_by = stonez_context_private.current_worker_initiator_id() AND id = stonez_context_private.current_worker_job_id()"
	policies := []policyMutation{
		{"queue SELECT", `ALTER POLICY bulk_import_jobs_signed_worker_select ON public.bulk_import_jobs USING (true)`, `ALTER POLICY bulk_import_jobs_signed_worker_select ON public.bulk_import_jobs USING (`+queue+`)`},
		{"queue UPDATE", `ALTER POLICY bulk_import_jobs_signed_worker_update ON public.bulk_import_jobs USING (true) WITH CHECK (true)`, `ALTER POLICY bulk_import_jobs_signed_worker_update ON public.bulk_import_jobs USING (`+queue+`) WITH CHECK (`+queue+`)`},
		{"verifier SELECT", `ALTER POLICY bulk_import_jobs_context_verifier_read ON public.bulk_import_jobs USING (false)`, `ALTER POLICY bulk_import_jobs_context_verifier_read ON public.bulk_import_jobs USING (true)`},
		{"verifier UPDATE", `ALTER POLICY bulk_import_jobs_context_verifier_lock ON public.bulk_import_jobs USING (false) WITH CHECK (false)`, `ALTER POLICY bulk_import_jobs_context_verifier_lock ON public.bulk_import_jobs USING (true) WITH CHECK (true)`},
		{"user role guard", `ALTER POLICY bulk_worker_user_role_guard ON public.users WITH CHECK (true)`, `ALTER POLICY bulk_worker_user_role_guard ON public.users WITH CHECK (stonez_context_private.bulk_worker_user_role_allowed(role))`},
	}
	for _, table := range []string{"users","students","teacher_profiles","parent_profiles","academic_sessions","school_classes","sections","student_enrollments","guardian_relationships"} {
		policies = append(policies, policyMutation{
			name: "SELECT "+table,
			weaken: "ALTER POLICY bulk_worker_signed_select ON public."+table+" USING (true)",
			restore: "ALTER POLICY bulk_worker_signed_select ON public."+table+" USING ("+tenant+")",
		})
	}
	for _, table := range []string{"users","students","teacher_profiles","parent_profiles","student_enrollments","guardian_relationships"} {
		policies = append(policies, policyMutation{
			name: "INSERT "+table,
			weaken: "ALTER POLICY bulk_worker_signed_insert ON public."+table+" WITH CHECK (true)",
			restore: "ALTER POLICY bulk_worker_signed_insert ON public."+table+" WITH CHECK ("+tenant+")",
		})
	}
	for _, policy := range policies {
		t.Run("policy_"+strings.ReplaceAll(policy.name," ","_"), func(t *testing.T) {
			if err := runnerDB.Exec(policy.weaken).Error; err != nil {
				t.Fatalf("weaken %s policy: %v", policy.name, err)
			}
			if err := ValidateBulkImportSignedContextReady(workerDB); err == nil {
				t.Fatalf("readiness accepted altered %s policy expression", policy.name)
			}
			if err := runnerDB.Exec(policy.restore).Error; err != nil {
				t.Fatalf("restore %s policy: %v", policy.name, err)
			}
			if err := ValidateBulkImportSignedContextReady(workerDB); err != nil {
				t.Fatalf("readiness rejected restored %s policy: %v", policy.name, err)
			}
		})
	}

	// An additional permissive policy assigned directly to the login must
	// also be detected; checking only policies assigned to the group is unsafe.
	if err := runnerDB.Exec(`CREATE POLICY bulk_worker_login_bypass_test ON public.students FOR SELECT TO stonez_bulk_worker_processing_test USING (true)`).Error; err != nil {
		t.Fatalf("create unexpected login policy: %v", err)
	}
	if err := ValidateBulkImportSignedContextReady(workerDB); err == nil {
		t.Fatal("readiness accepted an unexpected permissive policy assigned to the processing login")
	}
	if err := runnerDB.Exec(`DROP POLICY bulk_worker_login_bypass_test ON public.students`).Error; err != nil {
		t.Fatalf("drop unexpected login policy: %v", err)
	}
	if err := ValidateBulkImportSignedContextReady(workerDB); err != nil {
		t.Fatalf("readiness rejected after removing unexpected login policy: %v", err)
	}

	// Effective privilege checks must catch excess inherited and direct grants,
	// not just confirm that the installer granted the intended minimum.
	privilegeMutations := []struct{name, grant, revoke string}{
		{"table UPDATE", `GRANT UPDATE ON public.students TO stonez_bulk_worker_processing`, `REVOKE UPDATE ON public.students FROM stonez_bulk_worker_processing`},
		{"sensitive SELECT column", `GRANT SELECT (password_hash) ON public.users TO stonez_bulk_worker_processing`, `REVOKE SELECT (password_hash) ON public.users FROM stonez_bulk_worker_processing`},
		{"unexpected INSERT table", `GRANT INSERT ON public.academic_sessions TO stonez_bulk_worker_processing`, `REVOKE INSERT ON public.academic_sessions FROM stonez_bulk_worker_processing`},
		{"lease generation UPDATE", `GRANT UPDATE (lease_id) ON public.bulk_import_jobs TO stonez_bulk_worker_processing`, `REVOKE UPDATE (lease_id) ON public.bulk_import_jobs FROM stonez_bulk_worker_processing`},
		{"direct login UPDATE", `GRANT UPDATE ON public.students TO stonez_bulk_worker_processing_test`, `REVOKE UPDATE ON public.students FROM stonez_bulk_worker_processing_test`},
	}
	for _, mutation := range privilegeMutations {
		t.Run("privilege_"+strings.ReplaceAll(mutation.name," ","_"), func(t *testing.T) {
			if err := runnerDB.Exec(mutation.grant).Error; err != nil {
				t.Fatalf("grant excess %s privilege: %v", mutation.name, err)
			}
			if err := ValidateBulkImportSignedContextReady(workerDB); err == nil {
				t.Fatalf("readiness accepted excess %s privilege", mutation.name)
			}
			if err := runnerDB.Exec(mutation.revoke).Error; err != nil {
				t.Fatalf("revoke excess %s privilege: %v", mutation.name, err)
			}
			if err := ValidateBulkImportSignedContextReady(workerDB); err != nil {
				t.Fatalf("readiness rejected restored privileges after %s: %v", mutation.name, err)
			}
		})
	}

	// PostgreSQL 16+ membership SET options are distinct from INHERIT.
	// The processing login must inherit the policy group without being able
	// to SET ROLE into it.
	if err := runnerDB.Exec(`GRANT stonez_bulk_worker_processing TO stonez_bulk_worker_processing_test WITH ADMIN FALSE, INHERIT TRUE, SET TRUE`).Error; err != nil {
		t.Fatalf("enable SET ROLE for negative test: %v", err)
	}
	if err := ValidateBulkImportProcessingRole(workerDB); err == nil {
		t.Fatal("processing login with SET ROLE into worker group was accepted")
	}
	if err := runnerDB.Exec(`GRANT stonez_bulk_worker_processing TO stonez_bulk_worker_processing_test WITH ADMIN FALSE, INHERIT TRUE, SET FALSE`).Error; err != nil {
		t.Fatalf("restore no-SET-ROLE membership: %v", err)
	}
	if err := ValidateBulkImportProcessingRole(workerDB); err != nil {
		t.Fatalf("processing login rejected after restoring no-SET-ROLE membership: %v", err)
	}
}
