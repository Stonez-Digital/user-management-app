package service

import (
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestSchoolDashboardSchoolIsolationAndSummary(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file:school_dashboard_test?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err = db.AutoMigrate(
        &models.School{}, &models.User{}, &models.Student{}, &models.AcademicSession{}, &models.Term{},
        &models.SchoolClass{}, &models.Section{}, &models.Subject{}, &models.StudentEnrollment{},
        &models.TeacherAssignment{}, &models.AttendanceRecord{}, &models.Invoice{}, &models.InvoiceLine{},
        &models.Payment{}, &models.Assessment{}, &models.AssessmentResult{}, &models.AuditLog{},
        &models.GuardianRelationship{},
    ); err != nil { t.Fatal(err) }

    schoolA := models.School{Name: "School A", Code: "A", Status: models.SchoolStatusActive}
    schoolB := models.School{Name: "School B", Code: "B", Status: models.SchoolStatusActive}
    if err = db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&schoolB).Error; err != nil { t.Fatal(err) }

    adminA := models.User{Name: "Admin A", Email: "admin-a@test", Role: "school_admin", Active: true, SchoolID: &schoolA.ID}
    adminB := models.User{Name: "Admin B", Email: "admin-b@test", Role: "school_admin", Active: true, SchoolID: &schoolB.ID}
    teacherA := models.User{Name: "Teacher A", Email: "teacher-a@test", Role: "teacher", Active: true, SchoolID: &schoolA.ID}
    teacherB := models.User{Name: "Teacher B", Email: "teacher-b@test", Role: "teacher", Active: true, SchoolID: &schoolB.ID}
    parentA := models.User{Name: "Parent A", Email: "parent-a@test", Role: "parent", Active: true, SchoolID: &schoolA.ID}
    for _, user := range []*models.User{&adminA, &adminB, &teacherA, &teacherB, &parentA} {
        if err = db.Create(user).Error; err != nil { t.Fatal(err) }
    }

    sessionA := models.AcademicSession{SchoolID: schoolA.ID, Name: "2026/2027", StartDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC), Status: models.AcademicStatusActive}
    sessionB := models.AcademicSession{SchoolID: schoolB.ID, Name: "2026/2027", StartDate: sessionA.StartDate, EndDate: sessionA.EndDate, Status: models.AcademicStatusActive}
    if err = db.Create(&sessionA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&sessionB).Error; err != nil { t.Fatal(err) }
    termA := models.Term{SchoolID: schoolA.ID, AcademicSessionID: sessionA.ID, Name: "First", StartDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC), Status: models.AcademicStatusActive}
    termB := models.Term{SchoolID: schoolB.ID, AcademicSessionID: sessionB.ID, Name: "First", StartDate: termA.StartDate, EndDate: termA.EndDate, Status: models.AcademicStatusActive}
    if err = db.Create(&termA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&termB).Error; err != nil { t.Fatal(err) }

    classA := models.SchoolClass{SchoolID: schoolA.ID, Name: "JSS 1", Level: 1}
    classB := models.SchoolClass{SchoolID: schoolB.ID, Name: "JSS 1", Level: 1}
    if err = db.Create(&classA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&classB).Error; err != nil { t.Fatal(err) }
    sectionA := models.Section{SchoolID: schoolA.ID, ClassID: classA.ID, Name: "A"}
    sectionB := models.Section{SchoolID: schoolB.ID, ClassID: classB.ID, Name: "A"}
    if err = db.Create(&sectionA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&sectionB).Error; err != nil { t.Fatal(err) }

    studentUserA := models.User{Name: "Student A", Email: "student-a@test", Role: "student", Active: true, SchoolID: &schoolA.ID}
    studentUserB := models.User{Name: "Student B", Email: "student-b@test", Role: "student", Active: true, SchoolID: &schoolB.ID}
    if err = db.Create(&studentUserA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&studentUserB).Error; err != nil { t.Fatal(err) }
    studentA := models.Student{SchoolID: schoolA.ID, UserID: studentUserA.ID, AdmissionNumber: "A-001", EnrollmentStatus: models.EnrollmentActive}
    studentB := models.Student{SchoolID: schoolB.ID, UserID: studentUserB.ID, AdmissionNumber: "B-001", EnrollmentStatus: models.EnrollmentActive}
    if err = db.Create(&studentA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&studentB).Error; err != nil { t.Fatal(err) }

    enrollmentA := models.StudentEnrollment{SchoolID: schoolA.ID, StudentID: studentA.ID, AcademicSessionID: sessionA.ID, ClassID: classA.ID, SectionID: sectionA.ID, Status: models.EnrollmentStatusActive}
    if err = db.Create(&enrollmentA).Error; err != nil { t.Fatal(err) }

    subjectA := models.Subject{SchoolID: schoolA.ID, Code: "ENG", Name: "English", Active: true}
    if err = db.Create(&subjectA).Error; err != nil { t.Fatal(err) }
    assignmentA := models.TeacherAssignment{SchoolID: schoolA.ID, TeacherID: teacherA.ID, SubjectID: subjectA.ID, AcademicSessionID: sessionA.ID, TermID: termA.ID, ClassID: classA.ID, SectionID: &sectionA.ID, Active: true}
    if err = db.Create(&assignmentA).Error; err != nil { t.Fatal(err) }

    invoice := models.Invoice{SchoolID: schoolA.ID, InvoiceNumber: "INV-A-001", StudentEnrollmentID: enrollmentA.ID, TermID: termA.ID, DueDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), TotalAmount: 50000, PaidAmount: 20000, Balance: 30000, Status: models.InvoiceStatusPartiallyPaid}
    if err = db.Create(&invoice).Error; err != nil { t.Fatal(err) }

    audit := models.AuditLog{SchoolID: &schoolA.ID, ActorID: &adminA.ID, Action: "student.create", Resource: "student", ResourceID: &studentA.ID, CreatedAt: time.Now().UTC()}
    if err = db.Create(&audit).Error; err != nil { t.Fatal(err) }

    svc := NewSchoolDashboardService(db)
    got, err := svc.Get(schoolA.ID, adminA.ID, sessionA.ID, termA.ID)
    if err != nil { t.Fatal(err) }

    if got.School.Name != "School A" || got.School.Administrator.ID != adminA.ID { t.Fatalf("unexpected tenant identity: %+v", got.School) }
    if got.Overview.Students != 1 || got.Overview.ActiveEnrollments != 1 || got.Overview.Teachers != 1 || got.Overview.Parents != 1 {
        t.Fatalf("unexpected overview: %+v", got.Overview)
    }
    if got.Finance.TotalInvoiced != 50000 || got.Finance.AmountPaid != 20000 || got.Finance.OutstandingBalance != 30000 || got.Finance.OutstandingInvoices != 1 {
        t.Fatalf("unexpected finance: %+v", got.Finance)
    }
    if len(got.RecentActivity) != 1 || got.RecentActivity[0].ActorName != "Admin A" { t.Fatalf("unexpected activity: %+v", got.RecentActivity) }
    if got.AcademicContext.SessionID != sessionA.ID || got.AcademicContext.TermID != termA.ID { t.Fatalf("unexpected academic context: %+v", got.AcademicContext) }

    if _, err = svc.Get(schoolA.ID, adminB.ID, sessionA.ID, termA.ID); err == nil { t.Fatal("expected admin from another school to be rejected") }

    _ = uuid.Nil
}
