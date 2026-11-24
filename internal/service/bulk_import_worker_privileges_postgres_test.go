package service

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

type bulkImportPrivilegeColumns struct {
	selectColumns []string
	insertColumns []string
}

func TestBulkImportProcessingCandidatePrivilegesAgainstActualMigrations(t *testing.T) {
	db, schema, _, _, _ := openBulkWorkerPostgres(t)
	role := "bi_process_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := db.Exec("CREATE ROLE " + role + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS").Error; err != nil {
		t.Fatalf("create isolated processing role: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP OWNED BY " + role).Error
		_ = db.Exec("DROP ROLE " + role).Error
	})
	if err := db.Exec("GRANT USAGE ON SCHEMA " + schema + " TO " + role).Error; err != nil {
		t.Fatalf("grant isolated schema usage: %v", err)
	}

	// These are the exact read projections plus predicate columns used by the
	// current worker. They are candidate privileges, not production grants.
	candidates := map[string]bulkImportPrivilegeColumns{
		"users": {
			selectColumns: []string{"id", "school_id", "email", "role"},
		},
		"students": {
			selectColumns: []string{"id", "school_id", "admission_number"},
		},
		"teacher_profiles": {
			selectColumns: []string{"id", "school_id", "staff_id"},
		},
		"parent_profiles": {
			selectColumns: []string{"id", "school_id", "parent_identifier", "user_id"},
		},
		"academic_sessions": {
			selectColumns: []string{"id", "school_id", "name", "status"},
		},
		"school_classes": {
			selectColumns: []string{"id", "school_id", "name"},
		},
		"sections": {
			selectColumns: []string{"id", "school_id", "class_id", "name"},
		},
		"student_enrollments": {
			selectColumns: []string{"id", "school_id", "student_id", "academic_session_id", "status"},
		},
		"guardian_relationships": {
			selectColumns: []string{"id", "school_id", "guardian_user_id", "student_id"},
		},
	}

	now := time.Now().UTC()
	schoolID, userID := uuid.New(), uuid.New()
	schoolIDPtr := &schoolID
	insertModels := map[string]any{
		"users": &models.User{ID: userID, SchoolID: schoolIDPtr, Name: "Worker Test", Email: "worker-test@example.invalid", PasswordHash: "hashed", Role: "student", Active: true},
		"students": &models.Student{SchoolID: schoolID, UserID: userID, AdmissionNumber: "PRIV-001", Gender: "unspecified", GuardianName: "Guardian", GuardianPhone: "000", GuardianEmail: "guardian@example.invalid", EnrollmentStatus: models.EnrollmentActive},
		"teacher_profiles": &models.TeacherProfile{SchoolID: schoolID, UserID: userID, StaffID: "STAFF-001", Phone: "000", Gender: "unspecified", Department: "Test", Designation: "Teacher", Subjects: "Math", Classes: "JSS1", Status: "active"},
		"parent_profiles": &models.ParentProfile{SchoolID: schoolID, UserID: userID, ParentIdentifier: "PARENT-001", Phone: "000", Address: "Test address", Occupation: "Test"},
		"student_enrollments": &models.StudentEnrollment{SchoolID: schoolID, StudentID: uuid.New(), AcademicSessionID: uuid.New(), ClassID: uuid.New(), SectionID: uuid.New(), Status: models.EnrollmentStatusActive, EnrolledAt: now},
		"guardian_relationships": &models.GuardianRelationship{SchoolID: schoolID, GuardianUserID: userID, StudentID: uuid.New(), Relationship: "parent", Primary: true, Active: true},
	}

	// Generate INSERT SQL with the actual GORM models. The exact generated
	// column list—not a handwritten model-field guess—drives the candidate
	// INSERT column grants below.
	for table, model := range insertModels {
		stmt := db.Session(&gorm.Session{DryRun: true}).Create(model).Statement
		if stmt.Error != nil {
			t.Fatalf("generate %s INSERT SQL: %v", table, stmt.Error)
		}
		columns, err := generatedInsertColumns(stmt.SQL.String())
		if err != nil {
			t.Fatalf("parse generated %s INSERT SQL %q: %v", table, stmt.SQL.String(), err)
		}
		candidate := candidates[table]
		candidate.insertColumns = columns
		candidates[table] = candidate
	}

	// Generate the SELECT statements with the same projections and predicates
	// used in the worker. Include every predicate column in the privilege set.
	selectCases := []struct {
		table string
		model any
		columns []string
		where string
		args []any
	}{
		{"users", &models.User{}, []string{"id", "school_id", "email", "role"}, "LOWER(email) = ?", []any{"worker@example.invalid"}},
		{"students", &models.Student{}, []string{"id"}, "school_id = ? AND admission_number = ?", []any{schoolID, "PRIV-001"}},
		{"teacher_profiles", &models.TeacherProfile{}, []string{"id"}, "school_id = ? AND staff_id = ?", []any{schoolID, "STAFF-001"}},
		{"parent_profiles", &models.ParentProfile{}, []string{"user_id"}, "school_id = ? AND lower(trim(parent_identifier)) = ?", []any{schoolID, "parent-001"}},
		{"academic_sessions", &models.AcademicSession{}, []string{"id", "status"}, "school_id = ? AND name = ?", []any{schoolID, "2026/2027"}},
		{"school_classes", &models.SchoolClass{}, []string{"id"}, "school_id = ? AND lower(name) = lower(?)", []any{schoolID, "JSS1"}},
		{"sections", &models.Section{}, []string{"id"}, "school_id = ? AND class_id = ? AND name = ?", []any{schoolID, uuid.New(), "A"}},
		{"student_enrollments", &models.StudentEnrollment{}, []string{"id"}, "school_id = ? AND student_id = ? AND academic_session_id = ?", []any{schoolID, uuid.New(), uuid.New()}},
		{"guardian_relationships", &models.GuardianRelationship{}, []string{"id"}, "school_id = ? AND guardian_user_id = ? AND student_id = ?", []any{schoolID, uuid.New(), uuid.New()}},
	}
	for _, query := range selectCases {
		var destination any
		switch query.table {
		case "users": destination = &models.User{}
		case "students": destination = &models.Student{}
		case "teacher_profiles": destination = &models.TeacherProfile{}
		case "parent_profiles": destination = &models.ParentProfile{}
		case "academic_sessions": destination = &models.AcademicSession{}
		case "school_classes": destination = &models.SchoolClass{}
		case "sections": destination = &models.Section{}
		case "student_enrollments": destination = &models.StudentEnrollment{}
		case "guardian_relationships": destination = &models.GuardianRelationship{}
		}
		stmt := db.Session(&gorm.Session{DryRun: true}).Model(query.model).Select(query.columns).Where(query.where, query.args...).First(destination).Statement
		sqlText := stmt.SQL.String()
		if stmt.Error != nil || !strings.Contains(strings.ToLower(sqlText), "select") {
			t.Fatalf("generate %s SELECT SQL: sql=%q err=%v", query.table, sqlText, stmt.Error)
		}
		for _, column := range query.columns {
			if !strings.Contains(sqlText, "\""+column+"\"") {
				t.Fatalf("%s SELECT SQL does not project %q: %s", query.table, column, sqlText)
			}
		}
		candidate := candidates[query.table]
		for _, column := range query.columns {
			candidate.selectColumns = appendUnique(candidate.selectColumns, column)
		}
		// Predicate columns are separately explicit so the test does not
		// mistake a narrow projection for a narrow privilege requirement.
		for _, column := range selectPredicateColumns(query.table) {
			candidate.selectColumns = appendUnique(candidate.selectColumns, column)
		}
		candidates[query.table] = candidate
	}

	// The processing worker reads only these four queue predicate columns.
	queueSelect := db.Session(&gorm.Session{DryRun: true}).Model(&models.BulkImportJob{}).
		Select("id", "school_id", "initiated_by", "worker_id").
		Where("id = ? AND school_id = ? AND initiated_by = ? AND worker_id = ?", uuid.New(), schoolID, userID, uuid.New()).
		First(&models.BulkImportJob{}).Statement
	queueSelectSQL := queueSelect.SQL.String()
	if queueSelect.Error != nil || !strings.Contains(strings.ToLower(queueSelectSQL), "select") {
		t.Fatalf("generate queue predicate SELECT SQL: sql=%q err=%v", queueSelectSQL, queueSelect.Error)
	}
	for _, column := range []string{"id", "school_id", "initiated_by", "worker_id"} {
		if !strings.Contains(queueSelectSQL, "\""+column+"\"") {
			t.Fatalf("queue SELECT SQL does not project %q: %s", column, queueSelectSQL)
		}
	}
	for _, forbidden := range []string{"rows_json", "errors_json", "kind", "file_name", "lease_generation"} {
		if strings.Contains(queueSelectSQL, "\""+forbidden+"\"") {
			t.Fatalf("queue SELECT SQL unexpectedly includes %q: %s", forbidden, queueSelectSQL)
		}
	}

	// Queue progress is the only UPDATE path on these tables. Generate its
	// SQL from the worker's progress/error field set and grant only its SET
	// columns; WHERE columns are covered by the queue SELECT grant.
	queueUpdates := map[string]any{
		"status": models.BulkImportFailed,
		"created": 0,
		"skipped": 0,
		"failed": 1,
		"credentials_sent": 0,
		"credential_emails_failed": 0,
		"errors_json": "[]",
		"claimed_at": now,
		"worker_id": uuid.New(),
		"last_error": "test",
		"updated_at": now,
	}
	queueStmt := db.Session(&gorm.Session{DryRun: true}).Model(&models.BulkImportJob{}).
		Where("id = ? AND school_id = ? AND initiated_by = ? AND worker_id = ?", uuid.New(), schoolID, userID, uuid.New()).
		Updates(queueUpdates).Statement
	if queueStmt.Error != nil || !strings.Contains(strings.ToLower(queueStmt.SQL.String()), "update") {
		t.Fatalf("generate queue progress UPDATE SQL: sql=%q err=%v", queueStmt.SQL.String(), queueStmt.Error)
	}
	queueUpdateColumns, err := generatedUpdateColumns(queueStmt.SQL.String())
	if err != nil { t.Fatalf("parse generated queue UPDATE SQL %q: %v", queueStmt.SQL.String(), err) }

	// Verify the migrations really created the named tables/columns and record
	// the unresolved policy state. The existing helper applies database.Migrate
	// to a private schema before this test starts.
	tenantTables := make([]string, 0, len(candidates))
	for table := range candidates { tenantTables = append(tenantTables, table) }
	for _, table := range tenantTables {
		candidate := candidates[table]
		allColumns := append(append([]string{}, candidate.selectColumns...), candidate.insertColumns...)
		for _, column := range allColumns {
			var exists bool
			if err := db.Raw("SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = ? AND table_name = ? AND column_name = ?)", schema, table, column).Scan(&exists).Error; err != nil {
				t.Fatalf("inspect migrated column %s.%s: %v", table, column, err)
			}
			if !exists { t.Fatalf("actual application migrations did not create expected column %s.%s", table, column) }
		}
		var rlsEnabled, forceRLS bool
		if err := db.Raw("SELECT c.relrowsecurity, c.relforcerowsecurity FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=? AND c.relname=?", schema, table).Row().Scan(&rlsEnabled, &forceRLS); err != nil {
			t.Fatalf("inspect actual RLS flags for %s: %v", table, err)
		}
		wantRLS := table == "teacher_profiles" || table == "parent_profiles"
		if rlsEnabled != wantRLS || forceRLS {
			t.Fatalf("actual migration RLS flags for %s = enabled:%t forced:%t, want enabled:%t forced:false", table, rlsEnabled, forceRLS, wantRLS)
		}
		var policyCount int64
		if err := db.Raw("SELECT count(*) FROM pg_policy p JOIN pg_class c ON c.oid=p.polrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=? AND c.relname=?", schema, table).Scan(&policyCount).Error; err != nil {
			t.Fatalf("inspect actual migration policies for %s: %v", table, err)
		}
		if policyCount != 0 {
			t.Fatalf("expected no tenant policies from current application migrations on %s, found %d; review this test's baseline", table, policyCount)
		}
	}

	for table, candidate := range candidates {
		if err := grantColumnPrivileges(db, schema, role, table, "SELECT", candidate.selectColumns); err != nil {
			t.Fatalf("grant candidate SELECT columns on %s: %v", table, err)
		}
		if len(candidate.insertColumns) > 0 {
			if err := grantColumnPrivileges(db, schema, role, table, "INSERT", candidate.insertColumns); err != nil {
				t.Fatalf("grant candidate INSERT columns on %s: %v", table, err)
			}
		}
	}
	if err := grantColumnPrivileges(db, schema, role, "bulk_import_jobs", "SELECT", []string{"id", "school_id", "initiated_by", "worker_id"}); err != nil {
		t.Fatalf("grant queue predicate SELECT columns: %v", err)
	}
	if err := grantColumnPrivileges(db, schema, role, "bulk_import_jobs", "UPDATE", queueUpdateColumns); err != nil {
		t.Fatalf("grant queue progress UPDATE columns: %v", err)
	}

	for table, candidate := range candidates {
		for _, column := range candidate.selectColumns {
			assertColumnPrivilege(t, db, role, schema, table, column, "SELECT", true)
		}
		for _, column := range candidate.insertColumns {
			assertColumnPrivilege(t, db, role, schema, table, column, "INSERT", true)
		}
		for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
			var tablePrivilege bool
			if err := db.Raw("SELECT has_table_privilege(?, ?, ?)", role, schema+"."+table, privilege).Scan(&tablePrivilege).Error; err != nil {
				t.Fatalf("inspect table-level %s privilege on %s: %v", privilege, table, err)
			}
			if tablePrivilege { t.Fatalf("candidate role unexpectedly has table-level %s on %s", privilege, table) }
		}
	}
	for _, column := range []string{"rows_json", "errors_json", "kind", "file_name", "lease_generation"} {
		assertColumnPrivilege(t, db, role, schema, "bulk_import_jobs", column, "SELECT", false)
	}
	assertColumnPrivilege(t, db, role, schema, "users", "password_hash", "SELECT", false)
	assertColumnPrivilege(t, db, role, schema, "users", "password_hash", "INSERT", true)
	for _, column := range queueUpdateColumns {
		assertColumnPrivilege(t, db, role, schema, "bulk_import_jobs", column, "UPDATE", true)
	}
	assertColumnPrivilege(t, db, role, schema, "bulk_import_jobs", "rows_json", "UPDATE", false)
	for _, privilege := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
		var tablePrivilege bool
		if err := db.Raw("SELECT has_table_privilege(?, ?, ?)", role, schema+".bulk_import_jobs", privilege).Scan(&tablePrivilege).Error; err != nil {
			t.Fatalf("inspect queue table-level %s privilege: %v", privilege, err)
		}
		if tablePrivilege { t.Fatalf("candidate role unexpectedly has table-level %s on bulk_import_jobs", privilege) }
	}
}

func selectPredicateColumns(table string) []string {
	switch table {
	case "users": return []string{"email"}
	case "students": return []string{"school_id", "admission_number"}
	case "teacher_profiles": return []string{"school_id", "staff_id"}
	case "parent_profiles": return []string{"school_id", "parent_identifier"}
	case "academic_sessions": return []string{"school_id", "name"}
	case "school_classes": return []string{"school_id", "name"}
	case "sections": return []string{"school_id", "class_id", "name"}
	case "student_enrollments": return []string{"school_id", "student_id", "academic_session_id", "status"}
	case "guardian_relationships": return []string{"school_id", "guardian_user_id", "student_id"}
	default: return nil
	}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values { if existing == value { return values } }
	return append(values, value)
}

var insertColumnsPattern = regexp.MustCompile(`INSERT INTO [^\(]+\(([^\)]+)\) VALUES`)
var updateColumnsPattern = regexp.MustCompile(`SET (.+?) WHERE`)
var quotedIdentifierPattern = regexp.MustCompile(`"([^"]+)"`)

func generatedInsertColumns(sqlText string) ([]string, error) {
	match := insertColumnsPattern.FindStringSubmatch(sqlText)
	if len(match) != 2 { return nil, fmt.Errorf("could not locate INSERT column list") }
	matches := quotedIdentifierPattern.FindAllStringSubmatch(match[1], -1)
	columns := make([]string, 0, len(matches))
	for _, column := range matches { columns = append(columns, column[1]) }
	return columns, nil
}

func generatedUpdateColumns(sqlText string) ([]string, error) {
	match := updateColumnsPattern.FindStringSubmatch(sqlText)
	if len(match) != 2 { return nil, fmt.Errorf("could not locate UPDATE SET clause") }
	columns := quotedIdentifierPattern.FindAllStringSubmatch(match[1], -1)
	out := make([]string, 0, len(columns))
	for _, column := range columns { out = append(out, column[1]) }
	return out, nil
}

func grantColumnPrivileges(db *gorm.DB, schema, role, table, privilege string, columns []string) error {
	if len(columns) == 0 { return nil }
	quoted := make([]string, 0, len(columns))
	for _, column := range columns { quoted = append(quoted, "\""+column+"\"") }
	statement := "GRANT " + privilege + " (" + strings.Join(quoted, ", ") + ") ON TABLE " + schema + "." + table + " TO " + role
	return db.Exec(statement).Error
}

func assertColumnPrivilege(t *testing.T, db *gorm.DB, role, schema, table, column, privilege string, want bool) {
	t.Helper()
	var got bool
	query := "SELECT has_column_privilege(?, ?, ?, ?)"
	if err := db.Raw(query, role, schema+"."+table, column, privilege).Scan(&got).Error; err != nil {
		t.Fatalf("inspect %s.%s %s privilege: %v", table, column, privilege, err)
	}
	if got != want { t.Fatalf("%s.%s %s privilege = %t, want %t", table, column, privilege, got, want) }
}

func TestTeacherAndParentProfilesRequireCompositeSchoolUserForeignKeys(t *testing.T) {
	db, schema, _, _, _ := openBulkWorkerPostgres(t)

	for _, item := range []struct{ table, constraint string }{
		{"teacher_profiles", "teacher_profile_user_school_fk"},
		{"parent_profiles", "parent_profile_user_school_fk"},
	} {
		var count int64
		query := "SELECT count(*) FROM pg_constraint fk " +
			"JOIN pg_class child ON child.oid = fk.conrelid " +
			"JOIN pg_namespace child_ns ON child_ns.oid = child.relnamespace " +
			"WHERE fk.contype = 'f' AND child_ns.nspname = ? AND child.relname = ? AND fk.conname = ?"
		if err := db.Raw(query, schema, item.table, item.constraint).Scan(&count).Error; err != nil {
			t.Fatalf("inspect migrated %s constraint: %v", item.table, err)
		}
		if count != 1 { t.Fatalf("actual migrations installed %d copies of %s on %s; want exactly one", count, item.constraint, item.table) }
	}

	for _, table := range []string{"teacher_profiles", "parent_profiles"} {
		var count int64
		query := "SELECT count(*) FROM pg_constraint fk " +
			"JOIN pg_class child ON child.oid = fk.conrelid " +
			"JOIN pg_namespace child_ns ON child_ns.oid = child.relnamespace " +
			"JOIN pg_class parent ON parent.oid = fk.confrelid " +
			"JOIN pg_namespace parent_ns ON parent_ns.oid = parent.relnamespace " +
			"WHERE fk.contype = 'f' AND child_ns.nspname = ? AND child.relname = ? " +
			"AND parent_ns.nspname = ? AND parent.relname = 'users' " +
			"AND (SELECT array_agg(a.attname::text ORDER BY k.ordinality) FROM unnest(fk.conkey) WITH ORDINALITY AS k(attnum, ordinality) JOIN pg_attribute a ON a.attrelid = child.oid AND a.attnum = k.attnum) = ARRAY['school_id','user_id']::text[] " +
			"AND (SELECT array_agg(a.attname::text ORDER BY k.ordinality) FROM unnest(fk.confkey) WITH ORDINALITY AS k(attnum, ordinality) JOIN pg_attribute a ON a.attrelid = parent.oid AND a.attnum = k.attnum) = ARRAY['school_id','id']::text[]"
		if err := db.Raw(query, schema, table, schema).Scan(&count).Error; err != nil { t.Fatalf("inspect %s composite school/user FK: %v", table, err) }
		if count != 1 { t.Fatalf("%s has %d composite (school_id,user_id)->users(school_id,id) FKs; want exactly one", table, count) }
	}

	schools := []models.School{
		{Name: "Profile FK School A", Code: "PROFILE-FK-A", Status: models.SchoolStatusActive},
		{Name: "Profile FK School B", Code: "PROFILE-FK-B", Status: models.SchoolStatusActive},
	}
	if err := db.Create(&schools).Error; err != nil { t.Fatalf("create isolated schools: %v", err) }
	users := []models.User{
		{SchoolID: &schools[0].ID, Name: "Teacher A", Email: "teacher-a-profile-fk@example.invalid", PasswordHash: "hash", Role: "teacher", Active: true},
		{SchoolID: &schools[1].ID, Name: "Teacher B", Email: "teacher-b-profile-fk@example.invalid", PasswordHash: "hash", Role: "teacher", Active: true},
		{SchoolID: &schools[0].ID, Name: "Parent A", Email: "parent-a-profile-fk@example.invalid", PasswordHash: "hash", Role: "parent", Active: true},
		{SchoolID: &schools[1].ID, Name: "Parent B", Email: "parent-b-profile-fk@example.invalid", PasswordHash: "hash", Role: "parent", Active: true},
	}
	if err := db.Create(&users).Error; err != nil { t.Fatalf("create isolated profile users: %v", err) }

	validTeacher := models.TeacherProfile{SchoolID: schools[0].ID, UserID: users[0].ID, StaffID: "PROFILE-FK-VALID"}
	if err := db.Create(&validTeacher).Error; err != nil { t.Fatalf("same-school teacher profile should be accepted: %v", err) }
	invalidTeacher := models.TeacherProfile{SchoolID: schools[0].ID, UserID: users[1].ID, StaffID: "PROFILE-FK-CROSS"}
	if err := db.Create(&invalidTeacher).Error; err == nil { t.Fatal("cross-school teacher profile was accepted; migrated composite FK is ineffective") }

	validParent := models.ParentProfile{SchoolID: schools[0].ID, UserID: users[2].ID, ParentIdentifier: "PROFILE-PARENT-VALID"}
	if err := db.Create(&validParent).Error; err != nil { t.Fatalf("same-school parent profile should be accepted: %v", err) }
	invalidParent := models.ParentProfile{SchoolID: schools[0].ID, UserID: users[3].ID, ParentIdentifier: "PROFILE-PARENT-CROSS"}
	if err := db.Create(&invalidParent).Error; err == nil { t.Fatal("cross-school parent profile was accepted; migrated composite FK is ineffective") }
}


// TestProfileSchoolConsistencyMigrationFailsClosedAndCanBeRetried verifies
// that migration 48 rejects legacy cross-school rows, leaves no migration
// record or partial constraints behind, and succeeds after the data is fixed.
func TestProfileSchoolConsistencyMigrationFailsClosedAndCanBeRetried(t *testing.T) {
	db, schema, _, _, _ := openBulkWorkerPostgres(t)

	for _, item := range []struct{ table, constraint string }{
		{"teacher_profiles", "teacher_profile_user_school_fk"},
		{"parent_profiles", "parent_profile_user_school_fk"},
	} {
		if err := db.Exec("ALTER TABLE " + schema + "." + item.table + " DROP CONSTRAINT " + item.constraint).Error; err != nil {
			t.Fatalf("remove %s for legacy-data fixture: %v", item.constraint, err)
		}
	}
	if err := db.Exec("DELETE FROM " + schema + ".migrations WHERE version = 48").Error; err != nil {
		t.Fatalf("reset migration 48 for legacy-data fixture: %v", err)
	}

	schools := []models.School{
		{Name: "Legacy Profile School A", Code: "LEGACY-PROFILE-A", Status: models.SchoolStatusActive},
		{Name: "Legacy Profile School B", Code: "LEGACY-PROFILE-B", Status: models.SchoolStatusActive},
	}
	if err := db.Create(&schools).Error; err != nil {
		t.Fatalf("create legacy-data schools: %v", err)
	}
	users := []models.User{
		{SchoolID: &schools[0].ID, Name: "Legacy Teacher", Email: "legacy-teacher-profile@example.invalid", PasswordHash: "hash", Role: "teacher", Active: true},
		{SchoolID: &schools[1].ID, Name: "Legacy Parent", Email: "legacy-parent-profile@example.invalid", PasswordHash: "hash", Role: "parent", Active: true},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create legacy-data users: %v", err)
	}
	teacher := models.TeacherProfile{SchoolID: schools[0].ID, UserID: users[1].ID, StaffID: "LEGACY-CROSS-SCHOOL-TEACHER"}
	if err := db.Create(&teacher).Error; err != nil {
		t.Fatalf("seed cross-school teacher profile: %v", err)
	}
	parent := models.ParentProfile{SchoolID: schools[1].ID, UserID: users[0].ID, ParentIdentifier: "LEGACY-CROSS-SCHOOL-PARENT"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("seed cross-school parent profile: %v", err)
	}

	assertMigrationNotRecorded := func(stage string) {
		t.Helper()
		var count int64
		if err := db.Model(&database.Migration{}).Where("version = ?", 48).Count(&count).Error; err != nil {
			t.Fatalf("%s: inspect migration record: %v", stage, err)
		}
		if count != 0 {
			t.Fatalf("%s: failed migration 48 was recorded", stage)
		}
		var constraints int64
		if err := db.Raw("SELECT count(*) FROM pg_constraint WHERE conname IN ('teacher_profile_user_school_fk','parent_profile_user_school_fk')").Scan(&constraints).Error; err != nil {
			t.Fatalf("%s: inspect partial constraints: %v", stage, err)
		}
		if constraints != 0 {
			t.Fatalf("%s: failed migration left %d profile constraints behind", stage, constraints)
		}
	}

	err := database.Migrate(db)
	if err == nil || !strings.Contains(err.Error(), "teacher_profiles contains 1 rows") {
		t.Fatalf("migration should fail closed on the teacher profile mismatch, got %v", err)
	}
	assertMigrationNotRecorded("teacher mismatch")

	if err := db.Delete(&teacher).Error; err != nil {
		t.Fatalf("remove invalid teacher profile: %v", err)
	}
	err = database.Migrate(db)
	if err == nil || !strings.Contains(err.Error(), "parent_profiles contains 1 rows") {
		t.Fatalf("migration should fail closed on the parent profile mismatch, got %v", err)
	}
	assertMigrationNotRecorded("parent mismatch")

	if err := db.Delete(&parent).Error; err != nil {
		t.Fatalf("remove invalid parent profile: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migration should succeed after legacy profile rows are corrected: %v", err)
	}
	var migrationCount int64
	if err := db.Model(&database.Migration{}).Where("version = ?", 48).Count(&migrationCount).Error; err != nil {
		t.Fatalf("verify successful migration record: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("successful migration recorded %d times; want exactly one", migrationCount)
	}
}
