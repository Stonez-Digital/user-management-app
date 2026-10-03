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
    inactiveUserA := models.User{Name: "Former Student A", Email: "former-student-a@test", Role: "student", Active: false, SchoolID: &schoolA.ID}
    if err = db.Create(&studentA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&studentB).Error; err != nil { t.Fatal(err) }
    inactiveStudentA := models.Student{SchoolID: schoolA.ID, UserID: inactiveUserA.ID, AdmissionNumber: "A-OLD", EnrollmentStatus: models.EnrollmentInactive}
    if err = db.Create(&inactiveUserA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&inactiveStudentA).Error; err != nil { t.Fatal(err) }

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
    if got.Overview.Students != 2 || got.Overview.ActiveEnrollments != 1 || got.Overview.Teachers != 1 || got.Overview.Parents != 1 {
        t.Fatalf("unexpected overview: %+v", got.Overview)
    }
    if got.Finance.TotalInvoiced != 50000 || got.Finance.AmountPaid != 20000 || got.Finance.OutstandingBalance != 30000 || got.Finance.OutstandingInvoices != 1 {
        t.Fatalf("unexpected finance: %+v", got.Finance)
    }
    if len(got.RecentActivity) != 1 || got.RecentActivity[0].ActorName != "Admin A" { t.Fatalf("unexpected activity: %+v", got.RecentActivity) }
    if got.AcademicContext.SessionID != sessionA.ID || got.AcademicContext.TermID != termA.ID { t.Fatalf("unexpected academic context: %+v", got.AcademicContext) }
    if got.Health.Enrollment != "Complete" { t.Fatalf("inactive historical student should not make current enrollment incomplete: %+v", got.Health) }
    for _, alert := range got.Alerts { if alert.Key == "students_without_enrollment" { t.Fatalf("inactive historical student should not trigger enrollment alert: %+v", alert) } }


    if _, err = svc.Get(schoolA.ID, adminB.ID, sessionA.ID, termA.ID); err == nil { t.Fatal("expected admin from another school to be rejected") }

    _ = uuid.Nil
}


func TestSchoolDashboardFinanceReportingIsolatesSessionTermSchoolAndPaymentStatus(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file:school_dashboard_finance_reporting?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err = db.AutoMigrate(
        &models.School{}, &models.User{}, &models.Student{}, &models.AcademicSession{}, &models.Term{},
        &models.StudentEnrollment{}, &models.SchoolClass{}, &models.Section{}, &models.Subject{},
        &models.TeacherAssignment{}, &models.AttendanceRecord{}, &models.Invoice{}, &models.InvoiceLine{},
        &models.Payment{}, &models.Assessment{}, &models.AssessmentResult{}, &models.AuditLog{},
        &models.GuardianRelationship{},
    ); err != nil { t.Fatal(err) }

    schoolA := models.School{Name: "Finance A", Code: "FA", Status: models.SchoolStatusActive}
    schoolB := models.School{Name: "Finance B", Code: "FB", Status: models.SchoolStatusActive}
    if err = db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&schoolB).Error; err != nil { t.Fatal(err) }

    adminA := models.User{Name: "Admin A", Email: "finance-admin-a@test", Role: "school_admin", Active: true, SchoolID: &schoolA.ID}
    adminB := models.User{Name: "Admin B", Email: "finance-admin-b@test", Role: "school_admin", Active: true, SchoolID: &schoolB.ID}
    if err = db.Create(&adminA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&adminB).Error; err != nil { t.Fatal(err) }

    start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
    end := time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC)
    sessionA1 := models.AcademicSession{SchoolID: schoolA.ID, Name: "2026/2027", StartDate: start, EndDate: end, Status: models.AcademicStatusActive}
    sessionA2 := models.AcademicSession{SchoolID: schoolA.ID, Name: "2027/2028", StartDate: end.AddDate(0, 1, 1), EndDate: end.AddDate(1, 1, 1), Status: models.AcademicStatusActive}
    sessionB1 := models.AcademicSession{SchoolID: schoolB.ID, Name: "2026/2027", StartDate: start, EndDate: end, Status: models.AcademicStatusActive}
    for _, session := range []*models.AcademicSession{&sessionA1, &sessionA2, &sessionB1} {
        if err = db.Create(session).Error; err != nil { t.Fatal(err) }
    }

    first := models.Term{SchoolID: schoolA.ID, AcademicSessionID: sessionA1.ID, Name: "First", StartDate: start, EndDate: time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC), Status: models.AcademicStatusActive}
    second := models.Term{SchoolID: schoolA.ID, AcademicSessionID: sessionA1.ID, Name: "Second", StartDate: time.Date(2027, 1, 5, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2027, 4, 10, 0, 0, 0, 0, time.UTC), Status: models.AcademicStatusActive}
    nextSessionTerm := models.Term{SchoolID: schoolA.ID, AcademicSessionID: sessionA2.ID, Name: "First", StartDate: sessionA2.StartDate, EndDate: sessionA2.StartDate.AddDate(0, 3, 0), Status: models.AcademicStatusActive}
    foreignTerm := models.Term{SchoolID: schoolB.ID, AcademicSessionID: sessionB1.ID, Name: "First", StartDate: start, EndDate: time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC), Status: models.AcademicStatusActive}
    for _, term := range []*models.Term{&first, &second, &nextSessionTerm, &foreignTerm} {
        if err = db.Create(term).Error; err != nil { t.Fatal(err) }
    }

    userA := models.User{Name: "Student A", Email: "finance-student-a@test", Role: "student", Active: true, SchoolID: &schoolA.ID}
    userB := models.User{Name: "Student B", Email: "finance-student-b@test", Role: "student", Active: true, SchoolID: &schoolB.ID}
    if err = db.Create(&userA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&userB).Error; err != nil { t.Fatal(err) }
    studentA := models.Student{SchoolID: schoolA.ID, UserID: userA.ID, AdmissionNumber: "FA-001", EnrollmentStatus: models.EnrollmentActive}
    studentB := models.Student{SchoolID: schoolB.ID, UserID: userB.ID, AdmissionNumber: "FB-001", EnrollmentStatus: models.EnrollmentActive}
    if err = db.Create(&studentA).Error; err != nil { t.Fatal(err) }
    if err = db.Create(&studentB).Error; err != nil { t.Fatal(err) }

    enrollmentA1 := models.StudentEnrollment{SchoolID: schoolA.ID, StudentID: studentA.ID, AcademicSessionID: sessionA1.ID, Status: models.EnrollmentStatusActive}
    enrollmentA2 := models.StudentEnrollment{SchoolID: schoolA.ID, StudentID: studentA.ID, AcademicSessionID: sessionA2.ID, Status: models.EnrollmentStatusActive}
    enrollmentB1 := models.StudentEnrollment{SchoolID: schoolB.ID, StudentID: studentB.ID, AcademicSessionID: sessionB1.ID, Status: models.EnrollmentStatusActive}
    for _, enrollment := range []*models.StudentEnrollment{&enrollmentA1, &enrollmentA2, &enrollmentB1} {
        if err = db.Create(enrollment).Error; err != nil { t.Fatal(err) }
    }

    firstInvoice := models.Invoice{
        SchoolID: schoolA.ID, InvoiceNumber: "FA-FIRST", StudentEnrollmentID: enrollmentA1.ID, TermID: first.ID,
        DueDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), TotalAmount: 50000, PaidAmount: 20000, Balance: 30000,
        Status: models.InvoiceStatusPartiallyPaid,
    }
    secondInvoice := models.Invoice{
        SchoolID: schoolA.ID, InvoiceNumber: "FA-SECOND", StudentEnrollmentID: enrollmentA1.ID, TermID: second.ID,
        DueDate: time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC), TotalAmount: 100000, PaidAmount: 90000, Balance: 10000,
        Status: models.InvoiceStatusPartiallyPaid,
    }
    nextSessionInvoice := models.Invoice{
        SchoolID: schoolA.ID, InvoiceNumber: "FA-NEXT", StudentEnrollmentID: enrollmentA2.ID, TermID: nextSessionTerm.ID,
        DueDate: time.Date(2027, 10, 1, 0, 0, 0, 0, time.UTC), TotalAmount: 70000, PaidAmount: 70000, Balance: 0,
        Status: models.InvoiceStatusPaid,
    }
    foreignInvoice := models.Invoice{
        SchoolID: schoolB.ID, InvoiceNumber: "FB-FIRST", StudentEnrollmentID: enrollmentB1.ID, TermID: foreignTerm.ID,
        DueDate: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), TotalAmount: 90000, PaidAmount: 90000, Balance: 0,
        Status: models.InvoiceStatusPaid,
    }
    for _, invoice := range []*models.Invoice{&firstInvoice, &secondInvoice, &nextSessionInvoice, &foreignInvoice} {
        if err = db.Create(invoice).Error; err != nil { t.Fatal(err) }
    }

    succeededAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
    payments := []models.Payment{
        {SchoolID: schoolA.ID, InvoiceID: firstInvoice.ID, Amount: 20000, Provider: "test", Reference: "FA-SUCCEEDED", ReceiptNumber: "FA-R-1", Status: models.PaymentStatusSucceeded, PaidAt: &succeededAt},
        {SchoolID: schoolA.ID, InvoiceID: firstInvoice.ID, Amount: 10000, Provider: "test", Reference: "FA-PENDING", ReceiptNumber: "FA-R-2", Status: models.PaymentStatusPending},
        {SchoolID: schoolA.ID, InvoiceID: firstInvoice.ID, Amount: 5000, Provider: "test", Reference: "FA-FAILED", ReceiptNumber: "FA-R-3", Status: models.PaymentStatusFailed},
        {SchoolID: schoolA.ID, InvoiceID: firstInvoice.ID, Amount: 3000, Provider: "test", Reference: "FA-REFUNDED", ReceiptNumber: "FA-R-4", Status: models.PaymentStatusRefunded},
        {SchoolID: schoolA.ID, InvoiceID: secondInvoice.ID, Amount: 90000, Provider: "test", Reference: "FA-SECOND-PAY", ReceiptNumber: "FA-R-5", Status: models.PaymentStatusSucceeded, PaidAt: &succeededAt},
        {SchoolID: schoolB.ID, InvoiceID: foreignInvoice.ID, Amount: 90000, Provider: "test", Reference: "FB-SUCCEEDED", ReceiptNumber: "FB-R-1", Status: models.PaymentStatusSucceeded, PaidAt: &succeededAt},
    }
    if err = db.Create(&payments).Error; err != nil { t.Fatal(err) }

    svc := NewSchoolDashboardService(db)
    got, err := svc.Get(schoolA.ID, adminA.ID, sessionA1.ID, first.ID)
    if err != nil { t.Fatal(err) }

    if got.Finance.TotalInvoiced != 50000 {
        t.Fatalf("expected only selected term invoices, got total invoiced %v", got.Finance.TotalInvoiced)
    }
    if got.Finance.AmountPaid != 20000 {
        t.Fatalf("expected only succeeded payment amount reflected in invoice aggregate, got %v", got.Finance.AmountPaid)
    }
    if got.Finance.OutstandingBalance != 30000 || got.Finance.OutstandingInvoices != 1 {
        t.Fatalf("unexpected first-term outstanding finance: %+v", got.Finance)
    }
    if len(got.Finance.RecentInvoices) != 1 || got.Finance.RecentInvoices[0].InvoiceNumber != "FA-FIRST" {
        t.Fatalf("expected only first-term invoice, got %+v", got.Finance.RecentInvoices)
    }
    if len(got.Finance.RecentPayments) != 4 {
        t.Fatalf("expected all first-term payment statuses for recent-payment audit, got %d", len(got.Finance.RecentPayments))
    }

    gotNext, err := svc.Get(schoolA.ID, adminA.ID, sessionA2.ID, nextSessionTerm.ID)
    if err != nil { t.Fatal(err) }
    if gotNext.Finance.TotalInvoiced != 70000 || gotNext.Finance.AmountPaid != 70000 || gotNext.Finance.OutstandingBalance != 0 {
        t.Fatalf("next-session finance leaked or omitted records: %+v", gotNext.Finance)
    }

    if _, err = svc.Get(schoolA.ID, adminB.ID, sessionA1.ID, first.ID); err == nil {
        t.Fatal("expected cross-school administrator to be rejected")
    }
}
