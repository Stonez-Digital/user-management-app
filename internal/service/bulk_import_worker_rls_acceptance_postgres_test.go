package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/security/bulkcontext"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// acceptanceSigningKeySource is deliberately test-only. It supplies a fixed,
// disposable key and must never be used by a runtime worker.
type acceptanceSigningKeySource struct{ key bulkcontext.Key }

func (s acceptanceSigningKeySource) ActiveSigningKey(context.Context) (bulkcontext.Key, error) {
	return bulkcontext.Key{ID: s.key.ID, Secret: append([]byte(nil), s.key.Secret...)}, nil
}

// TestBulkImportWorkerTwoSchoolRLSAcceptance exercises the real worker row
// processors against the actual application migrations in a disposable
// PostgreSQL schema. Every role, grant and RLS policy installed here is a
// TEST-ONLY candidate fixture. It is not a production migration or approved
// tenant-policy implementation.
func TestBulkImportWorkerTwoSchoolRLSAcceptance(t *testing.T) {
	admin, schema, ownerRole, processGroup, _ := openBulkWorkerPostgres(t)
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	loginRole := "bi_accept_login_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	loginPassword := uuid.NewString() + uuid.NewString()
	privateSchema := "stonez_internal"

	// The shared worker test helper gives its queue API fixture broad table
	// grants for claim-function tests. Remove those grants before installing
	// the signed-context verifier so this acceptance test exercises only the
	// narrow queue column privileges from the operator script.
	if err := admin.Exec("REVOKE ALL PRIVILEGES ON TABLE " + schema + ".bulk_import_jobs FROM " + processGroup).Error; err != nil {
		t.Fatalf("remove broad test-helper queue grants: %v", err)
	}
	if err := admin.Exec("CREATE ROLE " + loginRole + " LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS PASSWORD '" + loginPassword + "'").Error; err != nil {
		t.Fatalf("create disposable restricted processing login: %v", err)
	}
	if err := admin.Exec("GRANT " + processGroup + " TO " + loginRole + " WITH INHERIT TRUE, SET FALSE").Error; err != nil {
		t.Fatalf("grant test processing group to login: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec("DROP OWNED BY " + loginRole).Error
		_ = admin.Exec("REVOKE " + processGroup + " FROM " + loginRole).Error
		_ = admin.Exec("DROP ROLE " + loginRole).Error
	})

	// The verifier's pgcrypto dependency is installed only in the disposable
	// test database. The operator SQL is rewritten to use this test's isolated
	// application schema and randomly named test roles.
	if err := admin.Exec("CREATE SCHEMA IF NOT EXISTS extensions").Error; err != nil {
		t.Fatalf("create disposable extensions schema: %v", err)
	}
	if err := admin.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions").Error; err != nil {
		t.Fatalf("install disposable pgcrypto extension: %v", err)
	}
	if err := admin.Exec("GRANT USAGE ON SCHEMA " + schema + " TO " + processGroup).Error; err != nil {
		t.Fatalf("grant candidate schema usage: %v", err)
	}

	scriptBytes, err := os.ReadFile("../../scripts/security/install_bulk_import_signed_context.sql")
	if err != nil {
		t.Fatalf("read signed-context installer: %v", err)
	}
	script := strings.ReplaceAll(string(scriptBytes), "\\set ON_ERROR_STOP on\n", "")
	script = strings.ReplaceAll(script, "stonez_bulk_context_owner", ownerRole)
	script = strings.ReplaceAll(script, "stonez_bulk_process_group", processGroup)
	script = strings.ReplaceAll(script, "stonez_internal", privateSchema)
	script = strings.ReplaceAll(script, "public", schema)
	if err := admin.Exec(script).Error; err != nil {
		t.Fatalf("install test-only signed-context verifier: %v", err)
	}

	// The verifier and current-context SECURITY DEFINER functions must continue
	// to work when the queue itself is forced through RLS. The dedicated owner
	// policies in the installer are the only intended path for those reads.
	if err := admin.Exec("ALTER TABLE " + schema + ".bulk_import_jobs FORCE ROW LEVEL SECURITY").Error; err != nil {
		t.Fatalf("force RLS on bulk_import_jobs: %v", err)
	}
	if err := admin.Exec("GRANT USAGE ON SCHEMA " + privateSchema + " TO " + processGroup).Error; err != nil {
		t.Fatalf("grant verifier function schema usage: %v", err)
	}

	// Candidate policy and column-grant fixture. It intentionally applies
	// FORCE RLS to every worker tenant table, including tables where the
	// application migrations currently leave RLS disabled. The policy shape
	// is a test hypothesis only and remains subject to separate review.
	type tableFixture struct {
		name          string
		model         any
		selectColumns []string
		allowInsert   bool
	}
	tables := []tableFixture{
		{"users", &models.User{}, []string{"id", "school_id", "email", "role"}, true},
		{"students", &models.Student{}, []string{"id", "school_id", "admission_number"}, true},
		{"teacher_profiles", &models.TeacherProfile{}, []string{"id", "school_id", "staff_id"}, true},
		{"parent_profiles", &models.ParentProfile{}, []string{"id", "school_id", "parent_identifier", "user_id"}, true},
		{"academic_sessions", &models.AcademicSession{}, []string{"id", "school_id", "name", "status"}, false},
		{"school_classes", &models.SchoolClass{}, []string{"id", "school_id", "name"}, false},
		{"sections", &models.Section{}, []string{"id", "school_id", "class_id", "name"}, false},
		{"student_enrollments", &models.StudentEnrollment{}, []string{"id", "school_id", "student_id", "academic_session_id", "status"}, true},
		{"guardian_relationships", &models.GuardianRelationship{}, []string{"id", "school_id", "guardian_user_id", "student_id"}, true},
	}
	for _, table := range tables {
		if len(table.selectColumns) == 0 {
			t.Fatalf("%s candidate SELECT privileges unexpectedly empty", table.name)
		}
		var insertColumns []string
		if table.allowInsert {
			stmt := admin.Session(&gorm.Session{DryRun: true}).Create(table.model).Statement
			if stmt.Error != nil {
				t.Fatalf("generate %s insert columns: %v", table.name, stmt.Error)
			}
			insertColumns, err = generatedInsertColumns(stmt.SQL.String())
			if err != nil {
				t.Fatalf("derive %s insert columns from GORM SQL: %v", table.name, err)
			}
			if len(insertColumns) == 0 {
				t.Fatalf("%s candidate INSERT privileges unexpectedly empty", table.name)
			}
		}
		quotedSelect := make([]string, 0, len(table.selectColumns))
		quotedInsert := make([]string, 0, len(insertColumns))
		for _, column := range table.selectColumns {
			quotedSelect = append(quotedSelect, fmt.Sprintf("%q", column))
		}
		for _, column := range insertColumns {
			quotedInsert = append(quotedInsert, fmt.Sprintf("%q", column))
		}
		if err := admin.Exec("GRANT SELECT (" + strings.Join(quotedSelect, ",") + ") ON TABLE " + schema + "." + table.name + " TO " + processGroup).Error; err != nil {
			t.Fatalf("grant candidate SELECT columns on %s: %v", table.name, err)
		}
		if table.allowInsert {
			if err := admin.Exec("GRANT INSERT (" + strings.Join(quotedInsert, ",") + ") ON TABLE " + schema + "." + table.name + " TO " + processGroup).Error; err != nil {
				t.Fatalf("grant candidate INSERT columns on %s: %v", table.name, err)
			}
		}
		if err := admin.Exec("ALTER TABLE " + schema + "." + table.name + " ENABLE ROW LEVEL SECURITY").Error; err != nil {
			t.Fatalf("enable candidate RLS on %s: %v", table.name, err)
		}
		if err := admin.Exec("ALTER TABLE " + schema + "." + table.name + " FORCE ROW LEVEL SECURITY").Error; err != nil {
			t.Fatalf("force candidate RLS on %s: %v", table.name, err)
		}
		policy := "CREATE POLICY " + table.name + "_bulk_context_select ON " + schema + "." + table.name +
			" FOR SELECT TO " + processGroup +
			" USING (EXISTS (SELECT 1 FROM " + privateSchema + ".current_bulk_import_context() ctx WHERE ctx.school_id = " + table.name + ".school_id))"
		if table.name == "users" {
			policy = "CREATE POLICY users_bulk_context_select ON " + schema + ".users FOR SELECT TO " + processGroup +
				" USING (school_id IS NOT NULL AND EXISTS (SELECT 1 FROM " + privateSchema + ".current_bulk_import_context() ctx WHERE ctx.school_id = users.school_id))"
		}
		if err := admin.Exec(policy).Error; err != nil {
			t.Fatalf("create test-only SELECT policy on %s: %v", table.name, err)
		}
		if table.allowInsert {
			insertPolicy := "CREATE POLICY " + table.name + "_bulk_context_insert ON " + schema + "." + table.name +
				" FOR INSERT TO " + processGroup +
				" WITH CHECK (EXISTS (SELECT 1 FROM " + privateSchema + ".current_bulk_import_context() ctx WHERE ctx.school_id = " + table.name + ".school_id))"
			if table.name == "users" {
				insertPolicy = "CREATE POLICY users_bulk_context_insert ON " + schema + ".users FOR INSERT TO " + processGroup +
					" WITH CHECK (school_id IS NOT NULL AND EXISTS (SELECT 1 FROM " + privateSchema + ".current_bulk_import_context() ctx WHERE ctx.school_id = users.school_id))"
			}
			if err := admin.Exec(insertPolicy).Error; err != nil {
				t.Fatalf("create test-only INSERT policy on %s: %v", table.name, err)
			}
		}
		// Restrictive policies are ANDed with every applicable permissive
		// policy, preventing a later broad policy from bypassing tenancy.
		guardPredicate := "EXISTS (SELECT 1 FROM " + privateSchema + ".current_bulk_import_context() ctx WHERE ctx.school_id = " + table.name + ".school_id)"
		if table.name == "users" {
			guardPredicate = "school_id IS NOT NULL AND EXISTS (SELECT 1 FROM " + privateSchema + ".current_bulk_import_context() ctx WHERE ctx.school_id = users.school_id)"
		}
		guard := "CREATE POLICY " + table.name + "_bulk_context_guard ON " + schema + "." + table.name +
			" AS RESTRICTIVE FOR ALL TO " + processGroup +
			" USING (" + guardPredicate + ") WITH CHECK (" + guardPredicate + ")"
		if err := admin.Exec(guard).Error; err != nil {
			t.Fatalf("create test-only restrictive tenant guard on %s: %v", table.name, err)
		}
	}
	// Deliberately simulate a future broad permissive policy. The restrictive
	// tenant guard above must still prevent cross-school visibility and writes.
	if err := admin.Exec("CREATE POLICY users_acceptance_broad_permissive ON " + schema + ".users AS PERMISSIVE FOR ALL TO " + processGroup + " USING (true) WITH CHECK (true)").Error; err != nil {
		t.Fatalf("create broad permissive regression policy: %v", err)
	}

	// Assert the test login inherits only the candidate role and has no direct
	// table-wide privileges. No role can assume the owner identity with SET ROLE.
	var roleAttributes struct {
		Login, Superuser, BypassRLS, CreateDB, CreateRole, CanSetGroup bool
	}
	if err := admin.Raw("SELECT r.rolcanlogin AS login, r.rolsuper AS superuser, r.rolbypassrls AS bypass_rls, r.rolcreatedb AS create_db, r.rolcreaterole AS create_role, pg_has_role(?, ?, 'SET') AS can_set_group FROM pg_roles r WHERE r.rolname = ?", loginRole, ownerRole, loginRole).Scan(&roleAttributes).Error; err != nil {
		t.Fatalf("inspect candidate login attributes: %v", err)
	}
	if !roleAttributes.Login || roleAttributes.Superuser || roleAttributes.BypassRLS || roleAttributes.CreateDB || roleAttributes.CreateRole || roleAttributes.CanSetGroup {
		t.Fatalf("candidate processing login has unsafe role attributes: %+v", roleAttributes)
	}
	for _, table := range tables {
		var tableWide bool
		if err := admin.Raw("SELECT has_table_privilege(?, ?, 'SELECT') OR has_table_privilege(?, ?, 'INSERT') OR has_table_privilege(?, ?, 'UPDATE') OR has_table_privilege(?, ?, 'DELETE')",
			loginRole, schema+"."+table.name, loginRole, schema+"."+table.name, loginRole, schema+"."+table.name, loginRole, schema+"."+table.name).Scan(&tableWide).Error; err != nil {
			t.Fatalf("inspect table-wide privileges on %s: %v", table.name, err)
		}
		if tableWide {
			t.Fatalf("candidate processing login has table-wide privileges on %s", table.name)
		}
		if !table.allowInsert {
			var anyInsert bool
			if err := admin.Raw("SELECT has_any_column_privilege(?, ?, 'INSERT')", loginRole, schema+"."+table.name).Scan(&anyInsert).Error; err != nil {
				t.Fatalf("inspect candidate INSERT privileges on read-only table %s: %v", table.name, err)
			}
			if anyInsert {
				t.Fatalf("candidate processing login unexpectedly has INSERT privilege on read-only table %s", table.name)
			}
		}
	}

	// The queue installer must not inherit the broad grants from the claim
	// helper. Its verifier grants only the context predicate columns and
	// progress UPDATE columns.
	var queueWide bool
	if err := admin.Raw("SELECT has_table_privilege(?, ?, 'SELECT') OR has_table_privilege(?, ?, 'UPDATE') OR has_table_privilege(?, ?, 'INSERT') OR has_table_privilege(?, ?, 'DELETE')",
		loginRole, schema+".bulk_import_jobs", loginRole, schema+".bulk_import_jobs", loginRole, schema+".bulk_import_jobs", loginRole, schema+".bulk_import_jobs").Scan(&queueWide).Error; err != nil {
		t.Fatalf("inspect queue table-wide privileges: %v", err)
	}
	if queueWide {
		t.Fatal("candidate processing login retained table-wide queue privileges")
	}

	// Connect as the actual LOGIN identity; SET ROLE is not used to simulate it.
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse test database DSN: %v", err)
	}
	parsed.User = url.UserPassword(loginRole, loginPassword)
	query := parsed.Query()
	query.Set("options", "-csearch_path="+schema+","+privateSchema+",extensions,public")
	parsed.RawQuery = query.Encode()
	loginDB, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect as restricted processing login: %v", err)
	}
	loginSQL, err := loginDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	loginSQL.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = loginSQL.Close() })
	var sessionUser, currentUser string
	if err := loginDB.Raw("SELECT session_user, current_user").Row().Scan(&sessionUser, &currentUser); err != nil {
		t.Fatalf("verify restricted connection identity: %v", err)
	}
	if sessionUser != loginRole || currentUser != loginRole {
		t.Fatalf("expected actual restricted login identity, got session=%q current=%q", sessionUser, currentUser)
	}

	key := bulkcontext.Key{ID: "acceptance-key", Secret: []byte("test-only-worker-signing-key-0123456789abcdef")}
	if err := admin.Exec("INSERT INTO "+privateSchema+".bulk_context_keys (key_id, secret, enabled) VALUES (?, ?, true)", key.ID, key.Secret).Error; err != nil {
		t.Fatalf("insert disposable verifier key: %v", err)
	}
	signer, err := bulkcontext.NewSigner(acceptanceSigningKeySource{key: key}, 2*time.Minute)
	if err != nil {
		t.Fatalf("create test-only signer: %v", err)
	}

	// Seed both schools, their initiating users, and the student-enrollment
	// reference data with the privileged fixture connection.
	type schoolFixture struct {
		school models.School
		initiator models.User
		session models.AcademicSession
		class models.SchoolClass
		section models.Section
		studentJob models.BulkImportJob
		teacherJob models.BulkImportJob
		parentJob models.BulkImportJob
		studentAdmission string
	}
	fixtures := make([]schoolFixture, 0, 2)
	for _, suffix := range []string{"a", "b"} {
		school := models.School{Name: "Acceptance School " + suffix, Code: "ACCEPT-" + strings.ToUpper(suffix), Status: models.SchoolStatusActive}
		if err := admin.Create(&school).Error; err != nil {
			t.Fatalf("create school %s: %v", suffix, err)
		}
		initiator := models.User{SchoolID: &school.ID, Name: "Initiator " + suffix, Email: "initiator-" + suffix + "@example.invalid", PasswordHash: "not-used", Role: "school_admin", Active: true}
		if err := admin.Create(&initiator).Error; err != nil {
			t.Fatalf("create school %s initiator: %v", suffix, err)
		}
		session := models.AcademicSession{SchoolID: school.ID, Name: "2026/2027", StartDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC), Status: models.AcademicStatusActive}
		if err := admin.Create(&session).Error; err != nil {
			t.Fatalf("create school %s academic session: %v", suffix, err)
		}
		class := models.SchoolClass{SchoolID: school.ID, Name: "JSS1", Level: 1}
		if err := admin.Create(&class).Error; err != nil {
			t.Fatalf("create school %s class: %v", suffix, err)
		}
		section := models.Section{SchoolID: school.ID, ClassID: class.ID, Name: "A"}
		if err := admin.Create(&section).Error; err != nil {
			t.Fatalf("create school %s section: %v", suffix, err)
		}
		workerID := uuid.New()
		now := time.Now().UTC()
		studentAdmission := "STU-" + strings.ToUpper(suffix)
		rows := map[string][]BulkRow{
			"students": {{ "name": "Student " + suffix, "email": "student-" + suffix + "@example.invalid", "admission_number": studentAdmission, "session": "2026/2027", "class": "JSS1", "section": "A" }},
			"teachers": {{ "name": "Teacher " + suffix, "email": "teacher-" + suffix + "@example.invalid", "staff_id": "STAFF-" + strings.ToUpper(suffix), "department": "Science" }},
			"parents": {{ "name": "Parent " + suffix, "email": "parent-" + suffix + "@example.invalid", "parent_identifier": "PARENT-" + strings.ToUpper(suffix), "student_admission_number": studentAdmission, "relationship": "parent", "primary": "yes" }},
		}
		jobs := make(map[string]models.BulkImportJob, 3)
		for kind, row := range rows {
			payload, err := json.Marshal(row)
			if err != nil {
				t.Fatalf("marshal %s rows for school %s: %v", kind, suffix, err)
			}
			job := models.BulkImportJob{
				SchoolID: school.ID, InitiatedBy: initiator.ID, Kind: kind,
				FileName: kind + "-" + suffix + ".json", Status: models.BulkImportRunning,
				Total: 1, Valid: 1, WorkerID: &workerID, ClaimedAt: &now,
				Attempts: 1, LeaseGeneration: 1, RowsJSON: string(payload), ErrorsJSON: "[]",
			}
			if err := admin.Create(&job).Error; err != nil {
				t.Fatalf("create %s job for school %s: %v", kind, suffix, err)
			}
			jobs[kind] = job
		}
		fixtures = append(fixtures, schoolFixture{
			school: school, initiator: initiator, session: session, class: class, section: section,
			studentJob: jobs["students"], teacherJob: jobs["teachers"], parentJob: jobs["parents"],
			studentAdmission: studentAdmission,
		})
	}

	service := (&BulkImportService{db: loginDB, contextSigner: signer})
	schoolA, schoolB := fixtures[0], fixtures[1]

	// Seed valid School B relationship targets for the cross-school INSERT
	// denial checks. Privileged fixture inserts ensure attempted processing
	// writes fail specifically at the tenant RLS WITH CHECK boundary.
	crossStudentUser := models.User{SchoolID: &schoolB.school.ID, Name: "Cross-target student", Email: "cross-target-student@example.invalid", PasswordHash: "not-used", Role: "student", Active: true}
	if err := admin.Create(&crossStudentUser).Error; err != nil { t.Fatalf("seed School B cross-target student user: %v", err) }
	crossStudent := models.Student{SchoolID: schoolB.school.ID, UserID: crossStudentUser.ID, AdmissionNumber: "RLS-CROSS-TARGET-STUDENT"}
	if err := admin.Create(&crossStudent).Error; err != nil { t.Fatalf("seed School B cross-target student: %v", err) }
	crossParentUser := models.User{SchoolID: &schoolB.school.ID, Name: "Cross-target parent", Email: "cross-target-parent@example.invalid", PasswordHash: "not-used", Role: "parent", Active: true}
	if err := admin.Create(&crossParentUser).Error; err != nil { t.Fatalf("seed School B cross-target parent user: %v", err) }
	crossParent := models.ParentProfile{SchoolID: schoolB.school.ID, UserID: crossParentUser.ID, ParentIdentifier: "RLS-CROSS-TARGET-PARENT"}
	if err := admin.Create(&crossParent).Error; err != nil { t.Fatalf("seed School B cross-target parent profile: %v", err) }
	crossEnrollment := models.StudentEnrollment{SchoolID: schoolB.school.ID, StudentID: crossStudent.ID, AcademicSessionID: schoolB.session.ID, ClassID: schoolB.class.ID, SectionID: schoolB.section.ID, Status: models.EnrollmentStatusActive}
	if err := admin.Create(&crossEnrollment).Error; err != nil { t.Fatalf("seed School B cross-target enrollment: %v", err) }
	crossGuardian := models.GuardianRelationship{SchoolID: schoolB.school.ID, GuardianUserID: crossParentUser.ID, StudentID: crossStudent.ID, Relationship: "parent", Primary: true, Active: true}
	if err := admin.Create(&crossGuardian).Error; err != nil { t.Fatalf("seed School B cross-target guardian relationship: %v", err) }

	// A global email collision owned by School B must remain invisible to the
	// restricted School A processing login. The attempted insert is rejected
	// by the real unique index, but the worker returns only a generic message.
	collisionEmail := "cross-school-collision@example.invalid"
	collisionUser := models.User{
		SchoolID: &schoolB.school.ID, Name: "Private School B account",
		Email: collisionEmail, PasswordHash: "not-used", Role: "student", Active: true,
	}
	if err := admin.Create(&collisionUser).Error; err != nil {
		t.Fatalf("seed School B global email collision: %v", err)
	}
	var collisionLookup []models.User
	if err := service.withTenantTransaction(schoolA.studentJob, func(tx *gorm.DB) error {
		return tx.Select("id").Where("LOWER(email) = ?", collisionEmail).Find(&collisionLookup).Error
	}); err != nil {
		t.Fatalf("query cross-school collision under School A context: %v", err)
	}
	if len(collisionLookup) != 0 {
		t.Fatalf("restricted School A context exposed %d cross-school email matches", len(collisionLookup))
	}
	created, skipped, _, _, _, collisionErr := service.processStudent(
		schoolA.studentJob, "Collision Attempt", collisionEmail,
		BulkRow{"admission_number": "COLLISION-A"},
	)
	if created || skipped || collisionErr == nil {
		t.Fatalf("cross-school email collision outcome unexpected: created=%t skipped=%t err=%v", created, skipped, collisionErr)
	}
	if collisionErr.Error() != bulkImportEmailUnavailableMessage {
		t.Fatalf("cross-school collision disclosed a specific reason: %q", collisionErr)
	}
	for _, sensitive := range []string{collisionEmail, schoolB.school.Code, "another tenant", "duplicate key", "unique constraint"} {
		if strings.Contains(strings.ToLower(collisionErr.Error()), strings.ToLower(sensitive)) {
			t.Fatalf("cross-school collision response disclosed %q: %q", sensitive, collisionErr)
		}
	}
	var collisionStudentCount int64
	if err := admin.Model(&models.Student{}).Where("school_id = ? AND admission_number = ?", schoolA.school.ID, "COLLISION-A").Count(&collisionStudentCount).Error; err != nil {
		t.Fatal(err)
	}
	if collisionStudentCount != 0 {
		t.Fatalf("cross-school email collision partially created %d School A student rows", collisionStudentCount)
	}

	// Neither an absent context nor a caller-forged GUC may expose tenant rows
	// or authorize inserts. The second case uses a syntactically valid UUID
	// that has no matching verifier session.
	for _, testCase := range []struct {
		name      string
		contextID string
	}{
		{name: "missing context"},
		{name: "forged context", contextID: uuid.NewString()},
	} {
		err := loginDB.Transaction(func(tx *gorm.DB) error {
			if testCase.contextID != "" {
				if err := tx.Exec("SELECT set_config('stonez.bulk_context_id', ?, true)", testCase.contextID).Error; err != nil {
					return err
				}
			}
			for _, table := range []string{"users", "students", "teacher_profiles", "parent_profiles", "academic_sessions", "school_classes", "sections", "student_enrollments", "guardian_relationships"} {
				var count int64
				if err := tx.Table(table).Where("school_id = ?", schoolA.school.ID).Count(&count).Error; err != nil {
					return fmt.Errorf("%s context queried %s: %w", testCase.name, table, err)
				}
				if count != 0 {
					return fmt.Errorf("%s exposed %d tenant rows in %s", testCase.name, count, table)
				}
			}
			if err := tx.SavePoint("before_unauthorized_user_insert").Error; err != nil {
				return err
			}
			unauthorized := models.User{
				SchoolID: &schoolA.school.ID, Name: "No context insert",
				Email: "no-context-" + testCase.name + "@example.invalid",
				PasswordHash: "not-used", Role: "student", Active: true,
			}
			if err := tx.Create(&unauthorized).Error; err == nil {
				return fmt.Errorf("%s authorized a tenant user insert", testCase.name)
			}
			return tx.RollbackTo("before_unauthorized_user_insert").Error
		})
		if err != nil {
			t.Fatalf("%s did not fail closed: %v", testCase.name, err)
		}
	}

	// A valid School A assertion must not reveal School B rows or permit a
	// cross-school profile insert, even though the candidate login has column
	// SELECT/INSERT grants on the underlying tables.
	if err := service.withTenantTransaction(schoolA.studentJob, func(tx *gorm.DB) error {
		for _, table := range []string{"users", "students", "teacher_profiles", "parent_profiles", "academic_sessions", "school_classes", "sections", "student_enrollments", "guardian_relationships"} {
			var count int64
			if err := tx.Table(table).Where("school_id = ?", schoolB.school.ID).Count(&count).Error; err != nil {
				return fmt.Errorf("query School B %s under School A context: %w", table, err)
			}
			if count != 0 {
				return fmt.Errorf("School A context saw %d School B rows in %s", count, table)
			}
		}
		if err := tx.SavePoint("before_cross_school_user").Error; err != nil {
			return err
		}
		crossSchoolUser := models.User{
			SchoolID: &schoolB.school.ID, Name: "Cross-school attempt",
			Email: "cross-school-attempt@example.invalid", PasswordHash: "not-used",
			Role: "student", Active: true,
		}
		if err := tx.Create(&crossSchoolUser).Error; err == nil {
			return fmt.Errorf("School A context inserted a School B user despite a broad permissive policy")
		}
		if err := tx.RollbackTo("before_cross_school_user").Error; err != nil {
			return err
		}
		if err := tx.SavePoint("before_cross_school_profile").Error; err != nil {
			return err
		}
		crossSchoolProfile := models.TeacherProfile{
			SchoolID: schoolB.school.ID, UserID: schoolB.initiator.ID,
			StaffID: "CROSS-SCHOOL-Attempt",
		}
		if err := tx.Create(&crossSchoolProfile).Error; err == nil {
			return fmt.Errorf("School A context inserted a School B teacher profile")
		}
		if err := tx.RollbackTo("before_cross_school_profile").Error; err != nil { return err }

		// Each target references valid School B rows so RLS is the rejection
		// boundary under test, rather than a missing-reference constraint.
		rejectCrossSchoolInsert := func(label, savepoint string, value any) error {
			if err := tx.SavePoint(savepoint).Error; err != nil { return err }
			insertErr := tx.Create(value).Error
			if insertErr == nil { return fmt.Errorf("School A context inserted a School B %s", label) }
			if !strings.Contains(strings.ToLower(insertErr.Error()), "row-level security") {
				return fmt.Errorf("School B %s insert failed for a reason other than RLS: %w", label, insertErr)
			}
			return tx.RollbackTo(savepoint).Error
		}
		crossSchoolStudent := models.Student{SchoolID: schoolB.school.ID, UserID: crossStudentUser.ID, AdmissionNumber: "RLS-CROSS-ATTEMPT-STUDENT"}
		if err := rejectCrossSchoolInsert("student", "before_cross_school_student", &crossSchoolStudent); err != nil { return err }
		crossSchoolParent := models.ParentProfile{SchoolID: schoolB.school.ID, UserID: crossParentUser.ID, ParentIdentifier: "RLS-CROSS-ATTEMPT-PARENT"}
		if err := rejectCrossSchoolInsert("parent profile", "before_cross_school_parent", &crossSchoolParent); err != nil { return err }
		crossSchoolEnrollment := models.StudentEnrollment{SchoolID: schoolB.school.ID, StudentID: crossStudent.ID, AcademicSessionID: schoolB.session.ID, ClassID: schoolB.class.ID, SectionID: schoolB.section.ID, Status: models.EnrollmentStatusActive}
		if err := rejectCrossSchoolInsert("student enrollment", "before_cross_school_enrollment", &crossSchoolEnrollment); err != nil { return err }
		crossSchoolGuardian := models.GuardianRelationship{SchoolID: schoolB.school.ID, GuardianUserID: crossParentUser.ID, StudentID: crossStudent.ID, Relationship: "parent", Primary: true, Active: true}
		if err := rejectCrossSchoolInsert("guardian relationship", "before_cross_school_guardian", &crossSchoolGuardian); err != nil { return err }
		return nil
	}); err != nil {
		t.Fatalf("two-school RLS isolation assertion failed: %v", err)
	}

	// Exercise verifier-level assertion rejection against the forced-RLS queue.
	// Each failure is isolated behind a savepoint so the surrounding transaction
	// remains usable for the next negative case.
	var transferredAssertion bulkcontext.Assertion
	err = loginDB.Transaction(func(tx *gorm.DB) error {
		identity, err := database.CurrentBulkImportTransactionIdentity(tx)
		if err != nil {
			return err
		}
		assertion, err := signer.Sign(tx.Statement.Context, bulkcontext.Binding{
			JobID: schoolA.studentJob.ID, SchoolID: schoolA.studentJob.SchoolID,
			InitiatedBy: schoolA.studentJob.InitiatedBy, WorkerID: *schoolA.studentJob.WorkerID,
			BackendPID: identity.BackendPID, TransactionID: identity.TransactionID,
			LeaseGeneration: schoolA.studentJob.LeaseGeneration,
		}, time.Minute)
		if err != nil {
			return err
		}
		invalid := assertion
		replacement := "A"
		if invalid.Signature[0] == 'A' {
			replacement = "B"
		}
		invalid.Signature = replacement + invalid.Signature[1:]
		if err := tx.SavePoint("before_invalid_assertion").Error; err != nil { return err }
		if err := database.VerifyBulkImportContext(tx, invalid); err == nil {
			return fmt.Errorf("invalid HMAC signature was accepted")
		}
		if err := tx.RollbackTo("before_invalid_assertion").Error; err != nil { return err }

		expired := assertion
		expired.Claims.ExpiresAt = time.Now().UTC().Unix() - 1
		expired.Claims.IssuedAt = expired.Claims.ExpiresAt - 60
		if err := tx.SavePoint("before_expired_assertion").Error; err != nil { return err }
		if err := database.VerifyBulkImportContext(tx, expired); err == nil {
			return fmt.Errorf("expired assertion was accepted")
		}
		if err := tx.RollbackTo("before_expired_assertion").Error; err != nil { return err }

		if err := database.VerifyBulkImportContext(tx, assertion); err != nil {
			return fmt.Errorf("valid assertion failed before replay test: %w", err)
		}
		if err := tx.SavePoint("before_replay_assertion").Error; err != nil { return err }
		if err := database.VerifyBulkImportContext(tx, assertion); err == nil {
			return fmt.Errorf("replayed assertion was accepted in the same transaction")
		}
		if err := tx.RollbackTo("before_replay_assertion").Error; err != nil { return err }
		transferredAssertion = assertion
		return nil
	})
	if err != nil {
		t.Fatalf("invalid/expired/replayed assertion acceptance checks failed: %v", err)
	}
	if err := loginDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.SavePoint("before_transferred_assertion").Error; err != nil { return err }
		if err := database.VerifyBulkImportContext(tx, transferredAssertion); err == nil {
			return fmt.Errorf("assertion transferred to another transaction was accepted")
		}
		return tx.RollbackTo("before_transferred_assertion").Error
	}); err != nil {
		t.Fatalf("transaction-transfer assertion rejection failed: %v", err)
	}

	// Queue progress updates must be bound to both the verified job and worker.
	// A School A context cannot update School B's job, and a different worker
	// identifier cannot satisfy the guarded UPDATE predicate for School A's job.
	var beforeSchoolB models.BulkImportJob
	if err := admin.First(&beforeSchoolB, "id = ?", schoolB.studentJob.ID).Error; err != nil { t.Fatal(err) }
	if err := service.withTenantTransaction(schoolA.studentJob, func(tx *gorm.DB) error {
		result := tx.Model(&models.BulkImportJob{}).Where("id = ?", schoolB.studentJob.ID).Update("created", 99)
		if result.Error != nil { return result.Error }
		if result.RowsAffected != 0 { return fmt.Errorf("School A context updated School B queue job") }
		return nil
	}); err != nil { t.Fatalf("cross-job queue progress update was not rejected: %v", err) }
	wrongProgressWorker := uuid.New()
	if err := service.updateClaimedJob(schoolA.studentJob, wrongProgressWorker, map[string]any{"created": 99}); err == nil {
		t.Fatal("queue progress update from a mismatched worker was accepted")
	}
	var afterSchoolB models.BulkImportJob
	if err := admin.First(&afterSchoolB, "id = ?", schoolB.studentJob.ID).Error; err != nil { t.Fatal(err) }
	if afterSchoolB.Created != beforeSchoolB.Created {
		t.Fatalf("mismatched job update changed School B progress: before=%d after=%d", beforeSchoolB.Created, afterSchoolB.Created)
	}
	var afterSchoolA models.BulkImportJob
	if err := admin.First(&afterSchoolA, "id = ?", schoolA.studentJob.ID).Error; err != nil { t.Fatal(err) }
	if afterSchoolA.Created != schoolA.studentJob.Created {
		t.Fatalf("mismatched worker update changed School A progress: before=%d after=%d", schoolA.studentJob.Created, afterSchoolA.Created)
	}

	// Negative lease checks: each assertion is signed from the supplied
	// job snapshot, while the verifier independently locks and checks the
	// authoritative queue row. No tenant callback may run on a stale snapshot.
	staleJob := schoolA.teacherJob
	if err := admin.Model(&models.BulkImportJob{}).Where("id = ?", staleJob.ID).
		Update("lease_generation", staleJob.LeaseGeneration+1).Error; err != nil {
		t.Fatalf("advance lease generation for stale-assertion test: %v", err)
	}
	staleCallbackRan := false
	staleErr := service.withTenantTransaction(staleJob, func(*gorm.DB) error {
		staleCallbackRan = true
		return nil
	})
	if staleErr == nil || staleCallbackRan {
		t.Fatalf("stale lease generation was not rejected before tenant callback: err=%v callback_ran=%t", staleErr, staleCallbackRan)
	}
	if err := admin.Model(&models.BulkImportJob{}).Where("id = ?", staleJob.ID).
		Update("lease_generation", staleJob.LeaseGeneration).Error; err != nil {
		t.Fatalf("restore lease generation after stale-assertion test: %v", err)
	}

	wrongWorkerJob := schoolA.teacherJob
	wrongWorker := uuid.New()
	wrongWorkerJob.WorkerID = &wrongWorker
	wrongWorkerCallbackRan := false
	wrongWorkerErr := service.withTenantTransaction(wrongWorkerJob, func(*gorm.DB) error {
		wrongWorkerCallbackRan = true
		return nil
	})
	if wrongWorkerErr == nil || wrongWorkerCallbackRan {
		t.Fatalf("different worker assertion was not rejected before tenant callback: err=%v callback_ran=%t", wrongWorkerErr, wrongWorkerCallbackRan)
	}

	expiredClaim := time.Now().UTC().Add(-6 * time.Minute)
	if err := admin.Model(&models.BulkImportJob{}).Where("id = ?", schoolA.parentJob.ID).
		Update("claimed_at", expiredClaim).Error; err != nil {
		t.Fatalf("expire claim timestamp for stale-claim test: %v", err)
	}
	expiredClaimCallbackRan := false
	expiredClaimErr := service.withTenantTransaction(schoolA.parentJob, func(*gorm.DB) error {
		expiredClaimCallbackRan = true
		return nil
	})
	if expiredClaimErr == nil || expiredClaimCallbackRan {
		t.Fatalf("expired claim was not rejected before tenant callback: err=%v callback_ran=%t", expiredClaimErr, expiredClaimCallbackRan)
	}
	if err := admin.Model(&models.BulkImportJob{}).Where("id = ?", schoolA.parentJob.ID).
		Update("claimed_at", time.Now().UTC()).Error; err != nil {
		t.Fatalf("restore claim timestamp after stale-claim test: %v", err)
	}

	// Exercise the actual production row-processing orchestration for every
	// supported import kind, first for School A and then for School B.
	for _, fixture := range fixtures {
		for _, job := range []models.BulkImportJob{fixture.studentJob, fixture.teacherJob, fixture.parentJob} {
			service.processClaimed(job, *job.WorkerID)
			var stored models.BulkImportJob
			if err := admin.Where("id = ?", job.ID).First(&stored).Error; err != nil {
				t.Fatalf("read completed %s job for school %s: %v", job.Kind, fixture.school.Code, err)
			}
			if stored.Status != models.BulkImportCompleted || stored.Created != 1 || stored.Failed != 0 {
				t.Fatalf("%s import for school %s did not complete cleanly: status=%s created=%d failed=%d last_error=%q errors=%s",
					job.Kind, fixture.school.Code, stored.Status, stored.Created, stored.Failed, stored.LastError, stored.ErrorsJSON)
			}
		}
	}

	for _, fixture := range fixtures {
		var students, teachers, parents, enrollments, guardians int64
		if err := admin.Model(&models.Student{}).Where("school_id = ?", fixture.school.ID).Count(&students).Error; err != nil { t.Fatal(err) }
		if err := admin.Model(&models.TeacherProfile{}).Where("school_id = ?", fixture.school.ID).Count(&teachers).Error; err != nil { t.Fatal(err) }
		if err := admin.Model(&models.ParentProfile{}).Where("school_id = ?", fixture.school.ID).Count(&parents).Error; err != nil { t.Fatal(err) }
		if err := admin.Model(&models.StudentEnrollment{}).Where("school_id = ?", fixture.school.ID).Count(&enrollments).Error; err != nil { t.Fatal(err) }
		if err := admin.Model(&models.GuardianRelationship{}).Where("school_id = ?", fixture.school.ID).Count(&guardians).Error; err != nil { t.Fatal(err) }
		if students != 1 || teachers != 1 || parents != 1 || enrollments != 1 || guardians != 1 {
			t.Fatalf("school %s import counts mismatch: students=%d teachers=%d parents=%d enrollments=%d guardians=%d",
				fixture.school.Code, students, teachers, parents, enrollments, guardians)
		}
	}

	// The login can persist password hashes for new users but cannot read them,
	// and it cannot inspect queue payloads or the verifier's key/session tables.
	var canReadPasswordHash, canInsertPasswordHash, canReadRowsJSON, canReadVerifierKeys, canReadVerifierSessions bool
	checks := "SELECT " +
		"has_column_privilege(current_user, '" + schema + ".users', 'password_hash', 'SELECT'), " +
		"has_column_privilege(current_user, '" + schema + ".users', 'password_hash', 'INSERT'), " +
		"has_column_privilege(current_user, '" + schema + ".bulk_import_jobs', 'rows_json', 'SELECT'), " +
		"has_table_privilege(current_user, '" + privateSchema + ".bulk_context_keys', 'SELECT'), " +
		"has_table_privilege(current_user, '" + privateSchema + ".bulk_context_sessions', 'SELECT')"
	row := loginDB.Raw(checks).Row()
	if err := row.Scan(&canReadPasswordHash, &canInsertPasswordHash, &canReadRowsJSON, &canReadVerifierKeys, &canReadVerifierSessions); err != nil {
		t.Fatalf("inspect restricted login sensitive-column privileges: %v", err)
	}
	if canReadPasswordHash || !canInsertPasswordHash || canReadRowsJSON || canReadVerifierKeys || canReadVerifierSessions {
		t.Fatalf("restricted login sensitive privileges incorrect: password_read=%v password_insert=%v payload_read=%v key_read=%v session_read=%v",
			canReadPasswordHash, canInsertPasswordHash, canReadRowsJSON, canReadVerifierKeys, canReadVerifierSessions)
	}
}
