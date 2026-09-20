package database

import (
    "fmt"
    "os"

    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/auth"
    "github.com/google/uuid"
    "gorm.io/driver/postgres"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func Connect() (*gorm.DB, error) {
    driver := os.Getenv("DB_DRIVER")
    if driver == "" { driver = "sqlite" }
    switch driver {
    case "postgres":
        dsn := os.Getenv("DATABASE_URL")
        if dsn == "" {
            dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
                envOrDefault("DB_HOST","localhost"), envOrDefault("DB_USER","postgres"), os.Getenv("DB_PASSWORD"),
                envOrDefault("DB_NAME","usersdb"), envOrDefault("DB_PORT","5432"), envOrDefault("DB_SSLMODE","require"))
        }
        db,err:=gorm.Open(postgres.Open(dsn),&gorm.Config{}); if err!=nil{return nil,fmt.Errorf("connect postgres database: %w",err)}; return db,nil
    case "sqlite":
        db,err:=gorm.Open(sqlite.Open(envOrDefault("DB_PATH","users.db")),&gorm.Config{}); if err!=nil{return nil,fmt.Errorf("connect sqlite database: %w",err)}; return db,nil
    default: return nil,fmt.Errorf("unsupported DB_DRIVER %q",driver)
    }
}
func envOrDefault(key,fallback string) string { if value:=os.Getenv(key); value!="" { return value }; return fallback }

func Migrate(db *gorm.DB) error {
    if !db.Migrator().HasTable(&Migration{}) {
        if err:=db.AutoMigrate(&Migration{});err!=nil{return fmt.Errorf("create migration table: %w",err)}
    }
    migrations:=[]MigrationStep{
        {Version:1,Name:"initial_school_management_schema",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.User{},&models.RefreshToken{},&models.Session{},&models.PasswordResetToken{},&models.RoleChangeAudit{},&models.AuditLog{})}},
        {Version:2,Name:"student_management",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.Student{})}},
        {Version:3,Name:"academic_foundation",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.AcademicSession{},&models.Term{})}},
        {Version:4,Name:"classes_and_sections",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.SchoolClass{},&models.Section{})}},
        {Version:5,Name:"student_enrollment",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.StudentEnrollment{})}},
        {Version:6,Name:"attendance_records",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.AttendanceRecord{})}},
        {Version:7,Name:"assessments",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.Assessment{})}},
        {Version:8,Name:"assessment_results",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.AssessmentResult{})}},
        {Version:9,Name:"finance",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.FeeItem{},&models.Invoice{},&models.InvoiceLine{},&models.Payment{})}},
        {Version:10,Name:"timetable",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.TimetableEntry{})}},
        {Version:11,Name:"guardian_relationships",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.GuardianRelationship{})}},
        {Version:12,Name:"notifications",Up:func(tx *gorm.DB) error{return tx.AutoMigrate(&models.Announcement{},&models.Notification{})}},
        {Version:13,Name:"hash_refresh_tokens",Up:func(tx *gorm.DB) error {
            if err:=tx.AutoMigrate(&models.RefreshToken{},&models.Session{});err!=nil{return err}
            var tokens []struct{ID uuid.UUID; Token string}
            if tx.Migrator().HasColumn(&models.RefreshToken{},"token") {
                if err:=tx.Raw("SELECT id, token FROM refresh_tokens WHERE token IS NOT NULL AND token <> ''").Scan(&tokens).Error;err!=nil{return err}
            }
            for _,token:=range tokens {
                if err:=tx.Model(&models.RefreshToken{}).Where("id = ?",token.ID).Update("token_hash",auth.HashRefreshToken(token.Token)).Error;err!=nil{return err}
            }
            var sessions []struct{ID uuid.UUID; RefreshToken string}
            if tx.Migrator().HasColumn(&models.Session{},"refresh_token") {
                if err:=tx.Raw("SELECT id, refresh_token FROM sessions WHERE refresh_token IS NOT NULL AND refresh_token <> ''").Scan(&sessions).Error;err!=nil{return err}
            }
            for _,session:=range sessions {
                if err:=tx.Model(&models.Session{}).Where("id = ?",session.ID).Update("refresh_token_hash",auth.HashRefreshToken(session.RefreshToken)).Error;err!=nil{return err}
            }
            if err:=tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens(token_hash)").Error;err!=nil{return err}
            if err:=tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_refresh_token_hash ON sessions(refresh_token_hash)").Error;err!=nil{return err}
            if tx.Migrator().HasColumn(&models.RefreshToken{},"token") { if err:=tx.Migrator().DropColumn(&models.RefreshToken{},"token");err!=nil{return err} }
            if tx.Migrator().HasColumn(&models.Session{},"refresh_token") { if err:=tx.Migrator().DropColumn(&models.Session{},"refresh_token");err!=nil{return err} }
            return nil
        }},
        {Version:14,Name:"refresh_token_families",Up:func(tx *gorm.DB) error {
            if err:=tx.AutoMigrate(&models.RefreshToken{},&models.Session{});err!=nil{return err}
            var sessions []models.Session
            if err:=tx.Find(&sessions).Error;err!=nil{return err}
            for _,session:=range sessions {
                if session.FamilyID != uuid.Nil {continue}
                familyID:=uuid.New()
                if session.RefreshTokenHash!="" {
                    var token models.RefreshToken
                    if err:=tx.Where("token_hash = ?",session.RefreshTokenHash).First(&token).Error;err==nil {
                        familyID=token.FamilyID
                        if familyID==uuid.Nil {
                            familyID=uuid.New()
                            if err:=tx.Model(&token).Update("family_id",familyID).Error;err!=nil{return err}
                        }
                    }
                }
                if err:=tx.Model(&session).Update("family_id",familyID).Error;err!=nil{return err}
            }
            var tokens []models.RefreshToken
            if err:=tx.Where("family_id = ?",uuid.Nil).Find(&tokens).Error;err!=nil{return err}
            for _,token:=range tokens {
                if err:=tx.Model(&token).Update("family_id",uuid.New()).Error;err!=nil{return err}
            }
            return nil
        }},
        {Version:15,Name:"hash_password_reset_tokens",Up:func(tx *gorm.DB) error {
            if err:=tx.AutoMigrate(&models.PasswordResetToken{});err!=nil{return err}
            var tokens []struct{ID uuid.UUID; Token string}
            if tx.Migrator().HasColumn(&models.PasswordResetToken{},"token") {
                if err:=tx.Raw("SELECT id, token FROM password_reset_tokens WHERE token IS NOT NULL AND token <> ''").Scan(&tokens).Error;err!=nil{return err}
                for _,token:=range tokens {
                    if err:=tx.Model(&models.PasswordResetToken{}).Where("id = ?",token.ID).Update("token_hash",auth.HashResetToken(token.Token)).Error;err!=nil{return err}
                }
                if err:=tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_password_reset_tokens_token_hash ON password_reset_tokens(token_hash)").Error;err!=nil{return err}
                if err:=tx.Migrator().DropColumn(&models.PasswordResetToken{},"token");err!=nil{return err}
            }
            return nil
        }},
        {Version:16,Name:"multi_school_tenant_foundation",Up:func(tx *gorm.DB) error {
            if err:=tx.AutoMigrate(&models.School{},&models.User{});err!=nil{return err}
            var school models.School
            result:=tx.Where("code = ?", "DEFAULT").First(&school)
            if result.Error==gorm.ErrRecordNotFound {
                school=models.School{Name:"Default School",Code:"DEFAULT",Status:models.SchoolStatusActive}
                if err:=tx.Create(&school).Error;err!=nil{return err}
            } else if result.Error!=nil {return result.Error}
            if err:=tx.Model(&models.User{}).Where("school_id IS NULL").Update("school_id",school.ID).Error;err!=nil{return err}
            if err:=tx.Exec("CREATE INDEX IF NOT EXISTS idx_users_school_id ON users(school_id)").Error;err!=nil{return err}
            return nil
        }},
        {Version:17,Name:"school_scope_academic_finance_data",Up:func(tx *gorm.DB) error {
            schoolScopedModels:=[]interface{}{
                &models.Student{}, &models.AcademicSession{}, &models.Term{},
                &models.SchoolClass{}, &models.Section{}, &models.Subject{},
                &models.StudentEnrollment{}, &models.AttendanceRecord{},
                &models.TeacherAssignment{}, &models.Assessment{}, &models.AssessmentResult{},
                &models.FeeItem{}, &models.Invoice{}, &models.InvoiceLine{}, &models.Payment{},
                &models.TimetableEntry{}, &models.GuardianRelationship{},
                &models.Announcement{}, &models.Notification{},
            }
            if tx.Dialector.Name()=="postgres" {
                tables:=[]string{"students","academic_sessions","terms","school_classes","sections","subjects","student_enrollments","attendance_records","teacher_assignments","assessments","assessment_results","fee_items","invoices","invoice_lines","payments","timetable_entries","guardian_relationships","announcements","notifications"}
                for _,table:=range tables {
                    if err:=tx.Exec("ALTER TABLE "+table+" ADD COLUMN IF NOT EXISTS school_id uuid").Error;err!=nil{return err}
                }
            } else {
                if err:=tx.AutoMigrate(schoolScopedModels...);err!=nil{return err}
            }

            var school models.School
            if err:=tx.Where("code = ?", "DEFAULT").First(&school).Error;err!=nil{return err}

            backfills:=[]string{
                "UPDATE students SET school_id = (SELECT school_id FROM users WHERE users.id = students.user_id) WHERE school_id IS NULL",
                "UPDATE academic_sessions SET school_id = ? WHERE school_id IS NULL",
                "UPDATE terms SET school_id = (SELECT school_id FROM academic_sessions WHERE academic_sessions.id = terms.academic_session_id) WHERE school_id IS NULL",
                "UPDATE school_classes SET school_id = ? WHERE school_id IS NULL",
                "UPDATE sections SET school_id = (SELECT school_id FROM school_classes WHERE school_classes.id = sections.class_id) WHERE school_id IS NULL",
                "UPDATE subjects SET school_id = ? WHERE school_id IS NULL",
                "UPDATE student_enrollments SET school_id = (SELECT school_id FROM students WHERE students.id = student_enrollments.student_id) WHERE school_id IS NULL",
                "UPDATE attendance_records SET school_id = (SELECT school_id FROM student_enrollments WHERE student_enrollments.id = attendance_records.enrollment_id) WHERE school_id IS NULL",
                "UPDATE teacher_assignments SET school_id = (SELECT school_id FROM users WHERE users.id = teacher_assignments.teacher_id) WHERE school_id IS NULL",
                "UPDATE assessments SET school_id = (SELECT school_id FROM teacher_assignments WHERE teacher_assignments.id = assessments.teacher_assignment_id) WHERE school_id IS NULL",
                "UPDATE assessment_results SET school_id = (SELECT school_id FROM student_enrollments WHERE student_enrollments.id = assessment_results.student_enrollment_id) WHERE school_id IS NULL",
                "UPDATE fee_items SET school_id = (SELECT school_id FROM terms WHERE terms.id = fee_items.term_id) WHERE school_id IS NULL",
                "UPDATE invoices SET school_id = (SELECT school_id FROM student_enrollments WHERE student_enrollments.id = invoices.student_enrollment_id) WHERE school_id IS NULL",
                "UPDATE invoice_lines SET school_id = (SELECT school_id FROM invoices WHERE invoices.id = invoice_lines.invoice_id) WHERE school_id IS NULL",
                "UPDATE payments SET school_id = (SELECT school_id FROM invoices WHERE invoices.id = payments.invoice_id) WHERE school_id IS NULL",
                "UPDATE timetable_entries SET school_id = (SELECT school_id FROM academic_sessions WHERE academic_sessions.id = timetable_entries.academic_session_id) WHERE school_id IS NULL",
                "UPDATE guardian_relationships SET school_id = (SELECT school_id FROM students WHERE students.id = guardian_relationships.student_id) WHERE school_id IS NULL",
                "UPDATE announcements SET school_id = (SELECT school_id FROM users WHERE users.id = announcements.created_by) WHERE school_id IS NULL",
                "UPDATE notifications SET school_id = (SELECT school_id FROM users WHERE users.id = notifications.user_id) WHERE school_id IS NULL",
            }
            for i,query:=range backfills {
                var err error
                if i==1 || i==3 || i==5 { err=tx.Exec(query,school.ID).Error } else { err=tx.Exec(query).Error }
                if err!=nil{return err}
            }

            required:=[]struct{table,column string}{
                {"students","school_id"},{"academic_sessions","school_id"},{"terms","school_id"},
                {"school_classes","school_id"},{"sections","school_id"},{"subjects","school_id"},
                {"student_enrollments","school_id"},{"attendance_records","school_id"},
                {"teacher_assignments","school_id"},{"assessments","school_id"},{"assessment_results","school_id"},
                {"fee_items","school_id"},{"invoices","school_id"},{"invoice_lines","school_id"},
                {"payments","school_id"},{"timetable_entries","school_id"},{"guardian_relationships","school_id"},
                {"announcements","school_id"},{"notifications","school_id"},
            }
            for _,item:=range required {
                var count int64
                if err:=tx.Table(item.table).Where(item.column+" IS NULL").Count(&count).Error;err!=nil{return err}
                if count!=0{return fmt.Errorf("%s.%s has %d unassigned rows after school backfill",item.table,item.column,count)}
            }

            drops:=[]string{
                "DROP INDEX IF EXISTS uni_students_admission_number",
                "DROP INDEX IF EXISTS idx_students_admission_number",
                "DROP INDEX IF EXISTS uni_academic_sessions_name",
                "DROP INDEX IF EXISTS idx_academic_sessions_name",
                "DROP INDEX IF EXISTS uni_school_classes_name",
                "DROP INDEX IF EXISTS idx_school_classes_name",
                "DROP INDEX IF EXISTS uni_subjects_code",
                "DROP INDEX IF EXISTS uni_subjects_name",
                "DROP INDEX IF EXISTS idx_subjects_code",
                "DROP INDEX IF EXISTS idx_subjects_name",
                "DROP INDEX IF EXISTS uq_session_term",
                "DROP INDEX IF EXISTS uq_fee_term_name",
                "DROP INDEX IF EXISTS uni_invoices_invoice_number",
                "DROP INDEX IF EXISTS idx_invoices_invoice_number",
                "DROP INDEX IF EXISTS uq_student_session",
                "DROP INDEX IF EXISTS uq_attendance_enrollment_date",
                "DROP INDEX IF EXISTS uq_assessment_assignment_title",
                "DROP INDEX IF EXISTS uq_result_assessment_enrollment",
                "DROP INDEX IF EXISTS ux_guardian_student",
                "DROP INDEX IF EXISTS uq_notification_user_source",
            }
            for _,query:=range drops { if err:=tx.Exec(query).Error;err!=nil{return err} }

            indexes:=[]string{
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_student_admission ON students(school_id, admission_number)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_session_name ON academic_sessions(school_id, name)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_session_term ON terms(school_id, academic_session_id, name)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_class_name ON school_classes(school_id, name)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_class_section ON sections(school_id, class_id, name)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_subject_code ON subjects(school_id, code)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_subject_name ON subjects(school_id, name)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_student_session ON student_enrollments(school_id, student_id, academic_session_id)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_attendance_enrollment_date ON attendance_records(school_id, enrollment_id, term_id, date)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_assessment_assignment_title ON assessments(school_id, teacher_assignment_id, title)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_result_assessment_enrollment ON assessment_results(school_id, assessment_id, student_enrollment_id)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_fee_term_name ON fee_items(school_id, term_id, name)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_invoice_number ON invoices(school_id, invoice_number)",
                "CREATE UNIQUE INDEX IF NOT EXISTS ux_school_guardian_student ON guardian_relationships(school_id, guardian_user_id, student_id)",
                "CREATE UNIQUE INDEX IF NOT EXISTS uq_school_notification_user_source ON notifications(school_id, user_id, announcement_id)",
            }
            for _,query:=range indexes { if err:=tx.Exec(query).Error;err!=nil{return err} }

            if tx.Dialector.Name()=="postgres" {
                for _,item:=range required {
                    if err:=tx.Exec("ALTER TABLE "+item.table+" ALTER COLUMN "+item.column+" SET NOT NULL").Error;err!=nil{return err}
                }
                constraints:=map[string]string{
                    "students":"student_school_fk",
                    "academic_sessions":"academic_session_school_fk",
                    "terms":"term_school_fk",
                    "school_classes":"school_class_school_fk",
                    "sections":"section_school_fk",
                    "subjects":"subject_school_fk",
                    "student_enrollments":"student_enrollment_school_fk",
                    "attendance_records":"attendance_record_school_fk",
                    "teacher_assignments":"teacher_assignment_school_fk",
                    "assessments":"assessment_school_fk",
                    "assessment_results":"assessment_result_school_fk",
                    "fee_items":"fee_item_school_fk",
                    "invoices":"invoice_school_fk",
                    "invoice_lines":"invoice_line_school_fk",
                    "payments":"payment_school_fk",
                    "timetable_entries":"timetable_entry_school_fk",
                    "guardian_relationships":"guardian_relationship_school_fk",
                    "announcements":"announcement_school_fk",
                    "notifications":"notification_school_fk",
                }
                for table,constraint:=range constraints {
                    if err:=tx.Exec("ALTER TABLE "+table+" ADD CONSTRAINT "+constraint+" FOREIGN KEY (school_id) REFERENCES schools(id) ON UPDATE CASCADE ON DELETE RESTRICT").Error;err!=nil{return err}
                }
            }
            return nil
        }},
    }
    for _,migration:=range migrations{
        var applied Migration
        result:=db.Where("version = ?",migration.Version).First(&applied)
        if result.Error==nil{continue}
        if result.Error!=gorm.ErrRecordNotFound{return fmt.Errorf("check migration %d: %w",migration.Version,result.Error)}
        if err:=db.Transaction(func(tx *gorm.DB) error{if err:=migration.Up(tx);err!=nil{return err};return tx.Create(&Migration{Version:migration.Version,Name:migration.Name}).Error});err!=nil{return fmt.Errorf("apply migration %d (%s): %w",migration.Version,migration.Name,err)}
    }
    return nil
}
type Migration struct{Version int `gorm:"primaryKey"`;Name string `gorm:"not null;uniqueIndex"`}
type MigrationStep struct{Version int;Name string;Up func(*gorm.DB) error}
