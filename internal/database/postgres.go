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
        {Version:18,Name:"database_tenant_integrity_constraints",Up:func(tx *gorm.DB) error {
            if tx.Dialector.Name()!="postgres" { return nil }

            checks:=[]struct{name,query string}{
                {"students.users","SELECT COUNT(*) FROM students s JOIN users u ON u.id=s.user_id WHERE s.school_id<>u.school_id"},
                {"terms.sessions","SELECT COUNT(*) FROM terms t JOIN academic_sessions s ON s.id=t.academic_session_id WHERE t.school_id<>s.school_id"},
                {"sections.classes","SELECT COUNT(*) FROM sections s JOIN school_classes c ON c.id=s.class_id WHERE s.school_id<>c.school_id"},
                {"enrollments.students","SELECT COUNT(*) FROM student_enrollments e JOIN students s ON s.id=e.student_id WHERE e.school_id<>s.school_id"},
                {"enrollments.sessions","SELECT COUNT(*) FROM student_enrollments e JOIN academic_sessions s ON s.id=e.academic_session_id WHERE e.school_id<>s.school_id"},
                {"enrollments.classes","SELECT COUNT(*) FROM student_enrollments e JOIN school_classes c ON c.id=e.class_id WHERE e.school_id<>c.school_id"},
                {"enrollments.sections","SELECT COUNT(*) FROM student_enrollments e JOIN sections s ON s.id=e.section_id WHERE e.school_id<>s.school_id"},
                {"attendance.enrollments","SELECT COUNT(*) FROM attendance_records a JOIN student_enrollments e ON e.id=a.enrollment_id WHERE a.school_id<>e.school_id"},
                {"attendance.terms","SELECT COUNT(*) FROM attendance_records a JOIN terms t ON t.id=a.term_id WHERE a.school_id<>t.school_id"},
                {"teacher_assignments.teachers","SELECT COUNT(*) FROM teacher_assignments ta JOIN users u ON u.id=ta.teacher_id WHERE ta.school_id<>u.school_id"},
                {"teacher_assignments.subjects","SELECT COUNT(*) FROM teacher_assignments ta JOIN subjects s ON s.id=ta.subject_id WHERE ta.school_id<>s.school_id"},
                {"teacher_assignments.sessions","SELECT COUNT(*) FROM teacher_assignments ta JOIN academic_sessions s ON s.id=ta.academic_session_id WHERE ta.school_id<>s.school_id"},
                {"teacher_assignments.terms","SELECT COUNT(*) FROM teacher_assignments ta JOIN terms t ON t.id=ta.term_id WHERE ta.school_id<>t.school_id"},
                {"teacher_assignments.classes","SELECT COUNT(*) FROM teacher_assignments ta JOIN school_classes c ON c.id=ta.class_id WHERE ta.school_id<>c.school_id"},
                {"teacher_assignments.sections","SELECT COUNT(*) FROM teacher_assignments ta JOIN sections s ON s.id=ta.section_id WHERE ta.section_id IS NOT NULL AND ta.school_id<>s.school_id"},
                {"assessments.assignments","SELECT COUNT(*) FROM assessments a JOIN teacher_assignments ta ON ta.id=a.teacher_assignment_id WHERE a.school_id<>ta.school_id"},
                {"results.assessments","SELECT COUNT(*) FROM assessment_results r JOIN assessments a ON a.id=r.assessment_id WHERE r.school_id<>a.school_id"},
                {"results.enrollments","SELECT COUNT(*) FROM assessment_results r JOIN student_enrollments e ON e.id=r.student_enrollment_id WHERE r.school_id<>e.school_id"},
                {"fee_items.terms","SELECT COUNT(*) FROM fee_items f JOIN terms t ON t.id=f.term_id WHERE f.school_id<>t.school_id"},
                {"invoices.enrollments","SELECT COUNT(*) FROM invoices i JOIN student_enrollments e ON e.id=i.student_enrollment_id WHERE i.school_id<>e.school_id"},
                {"invoices.terms","SELECT COUNT(*) FROM invoices i JOIN terms t ON t.id=i.term_id WHERE i.school_id<>t.school_id"},
                {"invoice_lines.invoices","SELECT COUNT(*) FROM invoice_lines l JOIN invoices i ON i.id=l.invoice_id WHERE l.school_id<>i.school_id"},
                {"invoice_lines.fee_items","SELECT COUNT(*) FROM invoice_lines l JOIN fee_items f ON f.id=l.fee_item_id WHERE l.fee_item_id IS NOT NULL AND l.school_id<>f.school_id"},
                {"payments.invoices","SELECT COUNT(*) FROM payments p JOIN invoices i ON i.id=p.invoice_id WHERE p.school_id<>i.school_id"},
                {"timetable.sessions","SELECT COUNT(*) FROM timetable_entries t JOIN academic_sessions s ON s.id=t.academic_session_id WHERE t.school_id<>s.school_id"},
                {"timetable.terms","SELECT COUNT(*) FROM timetable_entries t JOIN terms x ON x.id=t.term_id WHERE t.school_id<>x.school_id"},
                {"timetable.assignments","SELECT COUNT(*) FROM timetable_entries t JOIN teacher_assignments a ON a.id=t.teacher_assignment_id WHERE t.school_id<>a.school_id"},
                {"timetable.classes","SELECT COUNT(*) FROM timetable_entries t JOIN school_classes c ON c.id=t.class_id WHERE t.school_id<>c.school_id"},
                {"timetable.sections","SELECT COUNT(*) FROM timetable_entries t JOIN sections s ON s.id=t.section_id WHERE t.section_id IS NOT NULL AND t.school_id<>s.school_id"},
                {"guardians.users","SELECT COUNT(*) FROM guardian_relationships g JOIN users u ON u.id=g.guardian_user_id WHERE g.school_id<>u.school_id"},
                {"guardians.students","SELECT COUNT(*) FROM guardian_relationships g JOIN students s ON s.id=g.student_id WHERE g.school_id<>s.school_id"},
                {"announcements.users","SELECT COUNT(*) FROM announcements a JOIN users u ON u.id=a.created_by WHERE a.school_id<>u.school_id"},
                {"announcements.classes","SELECT COUNT(*) FROM announcements a JOIN school_classes c ON c.id=a.class_id WHERE a.class_id IS NOT NULL AND a.school_id<>c.school_id"},
                {"notifications.users","SELECT COUNT(*) FROM notifications n JOIN users u ON u.id=n.user_id WHERE n.school_id<>u.school_id"},
                {"notifications.announcements","SELECT COUNT(*) FROM notifications n JOIN announcements a ON a.id=n.announcement_id WHERE n.announcement_id IS NOT NULL AND n.school_id<>a.school_id"},
            }
            for _,check:=range checks {
                var count int64
                if err:=tx.Raw(check.query).Scan(&count).Error;err!=nil{return fmt.Errorf("check %s tenant consistency: %w",check.name,err)}
                if count!=0{return fmt.Errorf("tenant consistency violation in %s: %d rows",check.name,count)}
            }

            parentTables:=[]string{"users","students","academic_sessions","terms","school_classes","sections","subjects","student_enrollments","teacher_assignments","assessments","assessment_results","fee_items","invoices","announcements"}
            for _,table:=range parentTables {
                if err:=tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_"+table+"_school_id ON "+table+"(school_id,id)").Error;err!=nil{return err}
            }

            constraints:=[]struct{name,table,columns,parent string}{
                {"student_user_school_fk","students","school_id,user_id","users(school_id,id)"},
                {"term_session_school_fk","terms","school_id,academic_session_id","academic_sessions(school_id,id)"},
                {"section_class_school_fk","sections","school_id,class_id","school_classes(school_id,id)"},
                {"enrollment_student_school_fk","student_enrollments","school_id,student_id","students(school_id,id)"},
                {"enrollment_session_school_fk","student_enrollments","school_id,academic_session_id","academic_sessions(school_id,id)"},
                {"enrollment_class_school_fk","student_enrollments","school_id,class_id","school_classes(school_id,id)"},
                {"enrollment_section_school_fk","student_enrollments","school_id,section_id","sections(school_id,id)"},
                {"attendance_enrollment_school_fk","attendance_records","school_id,enrollment_id","student_enrollments(school_id,id)"},
                {"attendance_term_school_fk","attendance_records","school_id,term_id","terms(school_id,id)"},
                {"teacher_assignment_teacher_school_fk","teacher_assignments","school_id,teacher_id","users(school_id,id)"},
                {"teacher_assignment_subject_school_fk","teacher_assignments","school_id,subject_id","subjects(school_id,id)"},
                {"teacher_assignment_session_school_fk","teacher_assignments","school_id,academic_session_id","academic_sessions(school_id,id)"},
                {"teacher_assignment_term_school_fk","teacher_assignments","school_id,term_id","terms(school_id,id)"},
                {"teacher_assignment_class_school_fk","teacher_assignments","school_id,class_id","school_classes(school_id,id)"},
                {"teacher_assignment_section_school_fk","teacher_assignments","school_id,section_id","sections(school_id,id)"},
                {"assessment_assignment_school_fk","assessments","school_id,teacher_assignment_id","teacher_assignments(school_id,id)"},
                {"result_assessment_school_fk","assessment_results","school_id,assessment_id","assessments(school_id,id)"},
                {"result_enrollment_school_fk","assessment_results","school_id,student_enrollment_id","student_enrollments(school_id,id)"},
                {"fee_item_term_school_fk","fee_items","school_id,term_id","terms(school_id,id)"},
                {"invoice_enrollment_school_fk","invoices","school_id,student_enrollment_id","student_enrollments(school_id,id)"},
                {"invoice_term_school_fk","invoices","school_id,term_id","terms(school_id,id)"},
                {"invoice_line_invoice_school_fk","invoice_lines","school_id,invoice_id","invoices(school_id,id)"},
                {"invoice_line_fee_item_school_fk","invoice_lines","school_id,fee_item_id","fee_items(school_id,id)"},
                {"payment_invoice_school_fk","payments","school_id,invoice_id","invoices(school_id,id)"},
                {"timetable_session_school_fk","timetable_entries","school_id,academic_session_id","academic_sessions(school_id,id)"},
                {"timetable_term_school_fk","timetable_entries","school_id,term_id","terms(school_id,id)"},
                {"timetable_assignment_school_fk","timetable_entries","school_id,teacher_assignment_id","teacher_assignments(school_id,id)"},
                {"timetable_class_school_fk","timetable_entries","school_id,class_id","school_classes(school_id,id)"},
                {"timetable_section_school_fk","timetable_entries","school_id,section_id","sections(school_id,id)"},
                {"guardian_user_school_fk","guardian_relationships","school_id,guardian_user_id","users(school_id,id)"},
                {"guardian_student_school_fk","guardian_relationships","school_id,student_id","students(school_id,id)"},
                {"announcement_user_school_fk","announcements","school_id,created_by","users(school_id,id)"},
                {"announcement_class_school_fk","announcements","school_id,class_id","school_classes(school_id,id)"},
                {"notification_user_school_fk","notifications","school_id,user_id","users(school_id,id)"},
                {"notification_announcement_school_fk","notifications","school_id,announcement_id","announcements(school_id,id)"},
            }
            for _,fk:=range constraints {
                if err:=tx.Exec("ALTER TABLE "+fk.table+" ADD CONSTRAINT "+fk.name+" FOREIGN KEY ("+fk.columns+") REFERENCES "+fk.parent+" ON UPDATE CASCADE ON DELETE RESTRICT").Error;err!=nil{return fmt.Errorf("add %s: %w",fk.name,err)}
            }
            return nil
        }},
    },
    {Version:19,Name:"align_tenant_relationship_delete_semantics",Up:func(tx *gorm.DB) error {
        if tx.Dialector.Name()!="postgres" { return nil }
        changes:=[]struct{table,name,parent,onDelete string}{
            {"sections","section_class_school_fk","school_classes(school_id,id)","CASCADE"},
            {"terms","term_session_school_fk","academic_sessions(school_id,id)","CASCADE"},
            {"invoice_lines","invoice_line_invoice_school_fk","invoices(school_id,id)","CASCADE"},
        }
        for _,v:=range changes {
            if err:=tx.Exec("ALTER TABLE "+v.table+" DROP CONSTRAINT IF EXISTS "+v.name).Error;err!=nil{return err}
            if err:=tx.Exec("ALTER TABLE "+v.table+" ADD CONSTRAINT "+v.name+" FOREIGN KEY (school_id,"+map[string]string{"section_class_school_fk":"class_id","term_session_school_fk":"academic_session_id","invoice_line_invoice_school_fk":"invoice_id"}[v.name]+") REFERENCES "+v.parent+" ON UPDATE CASCADE ON DELETE "+v.onDelete).Error;err!=nil{return fmt.Errorf("recreate %s: %w",v.name,err)}
        }
        return nil
    }},
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
