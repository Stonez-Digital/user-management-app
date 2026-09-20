package database

import (
    "os"
    "strings"
    "testing"

    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func openTenantIntegrityPostgres(t *testing.T) *gorm.DB {
    t.Helper()
    dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
    if dsn == "" {
        t.Skip("TEST_DATABASE_URL is not configured")
    }
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        t.Fatal(err)
    }
    sqlDB, err := db.DB()
    if err != nil {
        t.Fatal(err)
    }
    t.Cleanup(func() { _ = sqlDB.Close() })
    return db
}

func TestPostgresTenantIntegrityMigration(t *testing.T) {
    db := openTenantIntegrityPostgres(t)

    // Verify the migration path is clean and idempotent on a fresh PostgreSQL database.
    if err := Migrate(db); err != nil {
        t.Fatal(err)
    }
    if err := Migrate(db); err != nil {
        t.Fatal(err)
    }

    var migrationCount int64
    if err := db.Model(&Migration{}).Count(&migrationCount).Error; err != nil {
        t.Fatal(err)
    }
    if migrationCount != 19 {
        t.Fatalf("expected 19 migrations, got %d", migrationCount)
    }

    var nullable int64
    if err := db.Raw(`
        SELECT COUNT(*)
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND column_name = 'school_id'
          AND is_nullable = 'YES'
          AND table_name IN (
            'students','academic_sessions','terms','school_classes','sections',
            'subjects','student_enrollments','attendance_records',
            'teacher_assignments','assessments','assessment_results',
            'fee_items','invoices','invoice_lines','payments',
            'timetable_entries','guardian_relationships','announcements',
            'notifications'
          )
    `).Scan(&nullable).Error; err != nil {
        t.Fatal(err)
    }
    if nullable != 0 {
        t.Fatalf("expected all tenant-owned school_id columns to be NOT NULL, found %d nullable columns", nullable)
    }

    var fkCount int64
    if err := db.Raw(`
        SELECT COUNT(*)
        FROM information_schema.table_constraints
        WHERE constraint_schema = current_schema()
          AND constraint_type = 'FOREIGN KEY'
          AND constraint_name LIKE '%_school_fk'
    `).Scan(&fkCount).Error; err != nil {
        t.Fatal(err)
    }
    if fkCount < 36 {
        t.Fatalf("expected composite tenant foreign keys, found %d", fkCount)
    }

    var first, second models.School
    if err := db.Where("code = ?", "DEFAULT").First(&first).Error; err != nil {
        t.Fatal(err)
    }
    second = models.School{Name: "Integrity Test School", Code: "INTEGRITY"}
    if err := db.Create(&second).Error; err != nil {
        t.Fatal(err)
    }

    user := models.User{
        SchoolID: &first.ID,
        Name: "Tenant Integrity User",
        Email: "tenant-integrity@example.com",
        Role: "student",
        Active: true,
    }
    if err := db.Create(&user).Error; err != nil {
        t.Fatal(err)
    }

    // The simple user_id FK is valid, but the composite tenant FK must reject
    // pairing a School B student with a School A user.
    crossSchool := models.Student{
        SchoolID:       second.ID,
        UserID:          user.ID,
        AdmissionNumber: "CROSS-001",
    }
    if err := db.Create(&crossSchool).Error; err == nil {
        t.Fatal("expected cross-school student/user relationship to be rejected")
    }

    valid := models.Student{
        SchoolID:       first.ID,
        UserID:          user.ID,
        AdmissionNumber: "VALID-001",
    }
    if err := db.Create(&valid).Error; err != nil {
        t.Fatalf("expected same-school relationship to succeed: %v", err)
    }

    if err := db.Delete(&first).Error; err == nil {
        t.Fatal("expected deleting a school with tenant-owned rows to be restricted")
    }
}
