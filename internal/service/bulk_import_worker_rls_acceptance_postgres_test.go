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
	}
	tables := []tableFixture{
		{"users", &models.User{}, []string{"id", "school_id", "email", "role"}},
		{"students", &models.Student{}, []string{"id", "school_id", "admission_number"}},
		{"teacher_profiles", &models.TeacherProfile{}, []string{"id", "school_id", "staff_id"}},
		{"parent_profiles", &models.ParentProfile{}, []string{"id", "school_id", "parent_identifier", "user_id"}},
		{"academic_sessions", &models.AcademicSession{}, []string{"id", "school_id", "name", "status"}},
		{"school_classes", &models.SchoolClass{}, []string{"id", "school_id", "name"}},
		{"sections", &models.Section{}, []string{"id", "school_id", "class_id", "name"}},
		{"student_enrollments", &models.StudentEnrollment{}, []string{"id", "school_id", "student_id", "academic_session_id", "status"}},
		{"guardian_relationships", &models.GuardianRelationship{}, []string{"id", "school_id", "guardian_user_id", "student_id"}},
	}
	for _, table := range tables {
		stmt := admin.Session(&gorm.Session{DryRun: true}).Create(table.model).Statement
		if stmt.Error != nil {
			t.Fatalf("generate %s insert columns: %v", table.name, stmt.Error)
		}
		insertColumns, err := generatedInsertColumns(stmt.SQL.String())
		if err != nil {
			t.Fatalf("derive %s insert columns from GORM SQL: %v", table.name, err)
		}
		if len(insertColumns) == 0 || len(table.selectColumns) == 0 {
			t.Fatalf("%s candidate privileges unexpectedly empty", table.name)
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
		if err := admin.Exec("GRANT INSERT (" + strings.Join(quotedInsert, ",") + ") ON TABLE " + schema + "." + table.name + " TO " + processGroup).Error; err != nil {
			t.Fatalf("grant candidate INSERT columns on %s: %v", table.name, err)
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
	// A valid School A assertion must not reveal School B rows or permit a
	// cross-school profile insert, even though the candidate login has column
	// SELECT/INSERT grants on the underlying tables.
	schoolA, schoolB := fixtures[0], fixtures[1]
	if err := service.withTenantTransaction(schoolA.studentJob, func(tx *gorm.DB) error {
		for _, table := range []string{"users", "students", "teacher_profiles", "parent_profiles"} {
			var count int64
			if err := tx.Table(table).Where("school_id = ?", schoolB.school.ID).Count(&count).Error; err != nil {
				return fmt.Errorf("query School B %s under School A context: %w", table, err)
			}
			if count != 0 {
				return fmt.Errorf("School A context saw %d School B rows in %s", count, table)
			}
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
		return tx.RollbackTo("before_cross_school_profile").Error
	}); err != nil {
		t.Fatalf("two-school RLS isolation assertion failed: %v", err)
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
