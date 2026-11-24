package service

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
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
