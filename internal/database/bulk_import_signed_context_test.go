package database

import (
	"bytes"
	"os"
	"strings"
	"time"
	"testing"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/security/bulkimportcontext"
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
		var aclDiagnostics []struct {
			FunctionName string
			OwnerName string
			ACL string
			WorkerCanExecute bool
			KeyAdminCanExecute bool
			PublicCanExecute bool
		}
		_ = workerDB.Raw(`SELECT p.oid::regprocedure::text AS function_name,
			owner_role.rolname AS owner_name,
			COALESCE(p.proacl::text, '<default>') AS acl,
			has_function_privilege('stonez_bulk_worker_processing', p.oid, 'EXECUTE') AS worker_can_execute,
			has_function_privilege('stonez_bulk_context_key_admin', p.oid, 'EXECUTE') AS key_admin_can_execute,
			has_function_privilege('public', p.oid, 'EXECUTE') AS public_can_execute
			FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
			JOIN pg_roles owner_role ON owner_role.oid=p.proowner
			WHERE n.nspname='stonez_context_private'
			ORDER BY p.proname`).Scan(&aclDiagnostics).Error
		t.Errorf("function ACL diagnostics: %+v", aclDiagnostics)
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

	// A replication-capable login must be rejected even when its other
	// role attributes and memberships are restricted.
	if err := runnerDB.Exec(`ALTER ROLE stonez_bulk_worker_processing_test REPLICATION`).Error; err != nil {
		t.Fatalf("enable replication attribute for negative test: %v", err)
	}
	if err := ValidateBulkImportProcessingRole(workerDB); err == nil {
		t.Fatal("replication-capable processing login was accepted")
	}
	if err := runnerDB.Exec(`ALTER ROLE stonez_bulk_worker_processing_test NOREPLICATION`).Error; err != nil {
		t.Fatalf("remove replication attribute after negative test: %v", err)
	}
	if err := ValidateBulkImportProcessingRole(workerDB); err != nil {
		t.Fatalf("processing login rejected after removing replication capability: %v", err)
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

	// ADMIN OPTION would let the processing login delegate its worker-group
	// membership to another role, so startup and the verifier must reject it.
	if err := runnerDB.Exec(`GRANT stonez_bulk_worker_processing TO stonez_bulk_worker_processing_test WITH ADMIN TRUE, INHERIT TRUE, SET FALSE`).Error; err != nil {
		t.Fatalf("enable worker-group ADMIN OPTION for negative test: %v", err)
	}
	if err := ValidateBulkImportProcessingRole(workerDB); err == nil {
		t.Fatal("processing login with worker-group ADMIN OPTION was accepted")
	}
	signer, err := bulkimportcontext.NewSigner("ci-key-1", bytes.Repeat([]byte{0xab}, 32), 30*time.Second)
	if err != nil {
		t.Fatalf("create verifier ADMIN OPTION regression signer: %v", err)
	}
	assertion, err := signer.Sign(bulkimportcontext.Context{
		JobID: uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		SchoolID: uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		InitiatedBy: uuid.MustParse("33333333-3333-4333-8333-333333333333"),
		WorkerID: uuid.MustParse("44444444-4444-4444-8444-444444444444"),
		LeaseID: uuid.MustParse("55555555-5555-4555-8555-555555555555"),
	})
	if err != nil {
		t.Fatalf("sign verifier ADMIN OPTION regression assertion: %v", err)
	}
	if err := workerDB.Exec("SELECT stonez_context_private.verify_bulk_import_context(?)", assertion).Error; err == nil {
		t.Fatal("PostgreSQL verifier accepted a processing login with worker-group ADMIN OPTION")
	}
	if err := runnerDB.Exec(`GRANT stonez_bulk_worker_processing TO stonez_bulk_worker_processing_test WITH ADMIN FALSE, INHERIT TRUE, SET FALSE`).Error; err != nil {
		t.Fatalf("restore worker-group membership without ADMIN OPTION: %v", err)
	}

	if err := ValidateBulkImportProcessingRole(workerDB); err != nil {
		t.Fatalf("processing login rejected after removing ADMIN OPTION: %v", err)
	}

	// The shell verifier workflow retires ci-key-1 during its rotation
	// checks before this Go integration test runs. Register an independent
	// live key so membership regressions reach the intended role boundary.
	if err := runnerDB.Exec("SELECT stonez_context_private.register_verification_key(?, decode(repeat('ab',32),'hex'), clock_timestamp() - interval '5 minutes', NULL)", "ci-membership-key").Error; err != nil {
		t.Fatalf("register membership regression key: %v", err)
	}
	membershipSigner, err := bulkimportcontext.NewSigner("ci-membership-key", bytes.Repeat([]byte{0xab}, 32), 30*time.Second)
	if err != nil { t.Fatalf("create membership regression signer: %v", err) }
	newAssertion := func() string {
		t.Helper()
		token, signErr := membershipSigner.Sign(bulkimportcontext.Context{
			JobID: uuid.MustParse("11111111-1111-4111-8111-111111111111"),
			SchoolID: uuid.MustParse("22222222-2222-4222-8222-222222222222"),
			InitiatedBy: uuid.MustParse("33333333-3333-4333-8333-333333333333"),
			WorkerID: uuid.MustParse("44444444-4444-4444-8444-444444444444"),
			LeaseID: uuid.MustParse("55555555-5555-4555-8555-555555555555"),
		})
		if signErr != nil { t.Fatalf("sign membership regression assertion: %v", signErr) }
		return token
	}
	assertVerifierRejects := func(label string) {
		t.Helper()
		if err := workerDB.Exec("SELECT stonez_context_private.verify_bulk_import_context(?)", newAssertion()).Error; err == nil {
			t.Fatalf("SQL verifier accepted forbidden membership: %s", label)
		}
	}

	// Privileged verifier roles are rejected by the database boundary, even
	// after startup validation has already succeeded.
	for _, roleName := range []string{"stonez_bulk_context_owner", "stonez_bulk_context_key_admin"} {
		if err := runnerDB.Exec("GRANT " + roleName + " TO stonez_bulk_worker_processing_test").Error; err != nil {
			t.Fatalf("grant forbidden %s membership for regression test: %v", roleName, err)
		}
		assertVerifierRejects("processing login inherits " + roleName)
		if err := ValidateBulkImportProcessingRole(workerDB); err == nil {
			t.Fatalf("startup validation accepted processing login membership in %s", roleName)
		}
		if err := runnerDB.Exec("REVOKE " + roleName + " FROM stonez_bulk_worker_processing_test").Error; err != nil {
			t.Fatalf("revoke forbidden %s membership: %v", roleName, err)
		}
		if err := ValidateBulkImportProcessingRole(workerDB); err != nil {
			t.Fatalf("processing login rejected after removing %s: %v", roleName, err)
		}
	}

	// A second login cannot join the processing group, even if the original
	// worker login passed startup validation earlier.
	if err := runnerDB.Exec("CREATE ROLE stonez_bulk_worker_processing_second_test LOGIN NOSUPERUSER NOBYPASSRLS INHERIT").Error; err != nil {
		t.Fatalf("create second processing login: %v", err)
	}
	if err := runnerDB.Exec("GRANT stonez_bulk_worker_processing TO stonez_bulk_worker_processing_second_test WITH ADMIN FALSE, INHERIT TRUE, SET FALSE").Error; err != nil {
		t.Fatalf("grant processing group to second login: %v", err)
	}
	assertVerifierRejects("second login is a processing-group member");
	if err := ValidateBulkImportProcessingRole(workerDB); err == nil {
		t.Fatal("startup validation accepted a second processing-group member")
	}
	if err := runnerDB.Exec("REVOKE stonez_bulk_worker_processing FROM stonez_bulk_worker_processing_second_test").Error; err != nil {
		t.Fatalf("revoke second processing login membership: %v", err)
	}
	if err := runnerDB.Exec("DROP ROLE stonez_bulk_worker_processing_second_test").Error; err != nil {
		t.Fatalf("drop second processing login: %v", err)
	}

	// Both directions of nested membership are forbidden: the login cannot
	// inherit an intermediate role, and the processing group cannot inherit
	// another role or accept an intermediate role as a member.
	if err := runnerDB.Exec("CREATE ROLE stonez_bulk_worker_processing_nested_test NOLOGIN").Error; err != nil {
		t.Fatalf("create nested membership role: %v", err)
	}
	if err := runnerDB.Exec("GRANT stonez_bulk_worker_processing_nested_test TO stonez_bulk_worker_processing_test WITH ADMIN FALSE, INHERIT TRUE, SET FALSE").Error; err != nil {
		t.Fatalf("grant nested role to processing login: %v", err)
	}
	assertVerifierRejects("processing login has nested extra membership");
	if err := ValidateBulkImportProcessingRole(workerDB); err == nil {
		t.Fatal("startup validation accepted nested processing-login membership")
	}
	if err := runnerDB.Exec("REVOKE stonez_bulk_worker_processing_nested_test FROM stonez_bulk_worker_processing_test").Error; err != nil {
		t.Fatalf("revoke nested login membership: %v", err)
	}
	// A privileged role inherited through an intermediate role must be
	// rejected just like a direct context-owner/key-admin membership.
	if err := runnerDB.Exec("GRANT stonez_bulk_context_owner TO stonez_bulk_worker_processing_nested_test").Error; err != nil {
		t.Fatalf("grant context-owner role to nested role: %v", err)
	}
	if err := runnerDB.Exec("GRANT stonez_bulk_worker_processing_nested_test TO stonez_bulk_worker_processing_test WITH ADMIN FALSE, INHERIT TRUE, SET FALSE").Error; err != nil {
		t.Fatalf("grant privileged nested role to processing login: %v", err)
	}
	assertVerifierRejects("processing login inherits context-owner through a nested role");
	if err := runnerDB.Exec("REVOKE stonez_bulk_worker_processing_nested_test FROM stonez_bulk_worker_processing_test").Error; err != nil {
		t.Fatalf("revoke privileged nested role from processing login: %v", err)
	}
	if err := runnerDB.Exec("REVOKE stonez_bulk_context_owner FROM stonez_bulk_worker_processing_nested_test").Error; err != nil {
		t.Fatalf("revoke context-owner role from nested role: %v", err)
	}
	if err := runnerDB.Exec("GRANT stonez_bulk_worker_processing TO stonez_bulk_worker_processing_nested_test WITH ADMIN FALSE, INHERIT TRUE, SET FALSE").Error; err != nil {
		t.Fatalf("grant processing group to nested role: %v", err)
	}
	assertVerifierRejects("nested role is a processing-group member");
	if err := runnerDB.Exec("REVOKE stonez_bulk_worker_processing FROM stonez_bulk_worker_processing_nested_test").Error; err != nil {
		t.Fatalf("revoke nested group membership: %v", err)
	}
	if err := runnerDB.Exec("GRANT stonez_bulk_worker_processing_nested_test TO stonez_bulk_worker_processing WITH ADMIN FALSE, INHERIT TRUE, SET FALSE").Error; err != nil {
		t.Fatalf("make processing group inherit nested role: %v", err)
	}
	assertVerifierRejects("processing group inherits another role");
	if err := runnerDB.Exec("REVOKE stonez_bulk_worker_processing_nested_test FROM stonez_bulk_worker_processing").Error; err != nil {
		t.Fatalf("revoke processing-group parent role: %v", err)
	}
	if err := runnerDB.Exec("DROP ROLE stonez_bulk_worker_processing_nested_test").Error; err != nil {
		t.Fatalf("drop nested membership role: %v", err)
	}
	if err := ValidateBulkImportProcessingRole(workerDB); err != nil {
		t.Fatalf("processing login rejected after restoring exclusive membership: %v", err)
	}

	// Membership is checked again by accessors within an already verified
	// transaction, not only once at startup or at assertion verification.
	accessorTx := workerDB.Begin()
	if accessorTx.Error != nil { t.Fatalf("begin accessor membership transaction: %v", accessorTx.Error) }
	if err := accessorTx.Exec("SELECT stonez_context_private.verify_bulk_import_context(?)", newAssertion()).Error; err != nil {
		_ = accessorTx.Rollback().Error
		t.Fatalf("verify assertion before live membership change: %v", err)
	}
	if err := runnerDB.Exec("GRANT stonez_bulk_context_key_admin TO stonez_bulk_worker_processing_test").Error; err != nil {
		_ = accessorTx.Rollback().Error
		t.Fatalf("grant key-admin role after startup and verification: %v", err)
	}
	var accessorResult struct { SchoolID *uuid.UUID }
	if err := accessorTx.Raw("SELECT stonez_context_private.current_worker_school_id() AS school_id").Scan(&accessorResult).Error; err != nil {
		t.Fatalf("call accessor after live membership change: %v", err)
	}
	if accessorResult.SchoolID != nil {
		t.Fatal("transaction-bound accessor retained tenant context after forbidden membership was added")
	}
	if err := runnerDB.Exec("REVOKE stonez_bulk_context_key_admin FROM stonez_bulk_worker_processing_test").Error; err != nil {
		t.Fatalf("revoke key-admin role after accessor regression: %v", err)
	}
	_ = accessorTx.Rollback().Error
	if err := ValidateBulkImportProcessingRole(workerDB); err != nil {
		t.Fatalf("processing login rejected after restoring membership: %v", err)
	}
}
