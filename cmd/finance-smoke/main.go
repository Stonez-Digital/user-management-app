package main

import (
    "fmt"
    "log"
    "os"
    "time"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "github.com/onoja217/users-management-app/internal/service"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func main() {
    dsn := os.Getenv("FINANCE_SMOKE_DATABASE_URL")
    if dsn == "" {
        log.Fatal("FINANCE_SMOKE_DATABASE_URL is required")
    }

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatalf("connect production database: %v", err)
    }

    tx := db.Begin()
    if tx.Error != nil {
        log.Fatalf("begin smoke transaction: %v", tx.Error)
    }
    defer tx.Rollback()

    // Everything created or changed by this smoke test lives inside one
    // transaction. The transaction is intentionally never committed.
    schoolID, term, enrollment, invoice, fee := fixture(tx)

    finance := service.NewFinanceService(
        repository.NewFeeItemRepository(tx),
        repository.NewInvoiceRepository(tx),
        repository.NewPaymentRepository(tx),
        tx,
    )

    // 1. Active enrollment + available session: invoice and payment must work.
    created, err := finance.CreateInvoice(
        schoolID,
        models.Invoice{
            StudentEnrollmentID: enrollment.ID,
            TermID: term.ID,
            DueDate: time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC),
        },
        []service.InvoiceLineInput{{
            FeeItemID:    &fee.ID,
            Description: "Tuition",
            Quantity:    1,
            UnitAmount:  50000,
        }},
    )
    must(err == nil, "active enrollment invoice creation failed: %v", err)
    must(created.TotalAmount == 50000 && created.Balance == 50000,
        "unexpected invoice totals: total=%v balance=%v", created.TotalAmount, created.Balance)

    paid, err := finance.CreatePayment(schoolID, models.Payment{
        InvoiceID: invoice.ID,
        Amount:    0,
        Provider:  "smoke",
        Reference: "unused",
    })
    _ = paid
    _ = err

    // Use the invoice returned by CreateInvoice for the positive payment path.
    _, err = finance.CreatePayment(schoolID, models.Payment{
        InvoiceID: created.ID,
        Amount:    50000,
        Provider:  "smoke",
        Reference: "FINANCE-SMOKE-" + uuid.NewString(),
        Status:    models.PaymentStatusSucceeded,
    })
    must(err == nil, "active enrollment payment creation failed: %v", err)

    // 2. Completed/withdrawn enrollment: payment must be rejected.
    for _, status := range []string{models.EnrollmentStatusCompleted, models.EnrollmentStatusWithdrawn} {
        must(tx.Model(&models.StudentEnrollment{}).
            Where("id = ?", enrollment.ID).
            Update("status", status).Error == nil,
            "set enrollment status %s", status)

        _, err = finance.CreatePayment(schoolID, models.Payment{
            InvoiceID: created.ID,
            Amount:    1,
            Provider:  "smoke",
            Reference: "FINANCE-SMOKE-" + uuid.NewString(),
            Status:    models.PaymentStatusSucceeded,
        })
        must(err == service.ErrPaymentInvalid,
            "expected payment rejection for %s enrollment, got %v", status, err)

        must(tx.Model(&models.StudentEnrollment{}).
            Where("id = ?", enrollment.ID).
            Update("status", models.EnrollmentStatusActive).Error == nil,
            "restore active enrollment after %s", status)
    }

    // 3. Closed/archived academic session: payment must be rejected.
    for _, status := range []string{models.AcademicStatusClosed, models.AcademicStatusArchived} {
        must(tx.Model(&models.AcademicSession{}).
            Where("id = ?", enrollment.AcademicSessionID).
            Update("status", status).Error == nil,
            "set session status %s", status)

        _, err = finance.CreatePayment(schoolID, models.Payment{
            InvoiceID: created.ID,
            Amount:    1,
            Provider:  "smoke",
            Reference: "FINANCE-SMOKE-" + uuid.NewString(),
            Status:    models.PaymentStatusSucceeded,
        })
        must(err == service.ErrPaymentInvalid,
            "expected payment rejection for %s session, got %v", status, err)

        must(tx.Model(&models.AcademicSession{}).
            Where("id = ?", enrollment.AcademicSessionID).
            Update("status", models.AcademicStatusActive).Error == nil,
            "restore active session after %s", status)
    }

    // 4. Fee-item amount manipulation: invoice must be rejected.
    _, err = finance.CreateInvoice(
        schoolID,
        models.Invoice{
            StudentEnrollmentID: enrollment.ID,
            TermID: term.ID,
            DueDate: time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC),
        },
        []service.InvoiceLineInput{{
            FeeItemID:    &fee.ID,
            Description: "Tuition",
            Quantity:    1,
            UnitAmount:  1,
        }},
    )
    must(err == service.ErrInvoiceInvalid,
        "expected manipulated fee amount rejection, got %v", err)

    log.Printf("FINANCE_SMOKE_PASS active-invoice active-payment inactive-enrollment session-lifecycle fee-integrity")
}

func fixture(tx *gorm.DB) (uuid.UUID, models.Term, models.StudentEnrollment, models.Invoice, models.FeeItem) {
    suffix := uuid.NewString()[:8]
    school := models.School{
        Name:   "Production Finance Smoke " + suffix,
        Code:   "FINSMOKE-" + suffix,
        Status: models.SchoolStatusActive,
    }
    must(tx.Create(&school).Error == nil, "create smoke school")

    user := models.User{
        SchoolID: &school.ID,
        Name:     "Finance Smoke Student",
        Email:    "finance-smoke-" + suffix + "@example.invalid",
        Role:     "student",
        Active:   true,
    }
    must(tx.Create(&user).Error == nil, "create smoke user")

    student := models.Student{
        SchoolID:        school.ID,
        UserID:          user.ID,
        AdmissionNumber: "FINSMOKE-" + suffix,
    }
    must(tx.Create(&student).Error == nil, "create smoke student")

    session := models.AcademicSession{
        SchoolID:  school.ID,
        Name:      "Smoke Session " + suffix,
        StartDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
        EndDate:   time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC),
        Status:    models.AcademicStatusActive,
    }
    must(tx.Create(&session).Error == nil, "create smoke session")

    term := models.Term{
        SchoolID:          school.ID,
        AcademicSessionID: session.ID,
        Name:              "Smoke Term " + suffix,
        StartDate:         time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
        EndDate:           time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC),
        Status:            models.AcademicStatusActive,
    }
    must(tx.Create(&term).Error == nil, "create smoke term")

    class := models.SchoolClass{
        SchoolID: school.ID,
        Name:     "Smoke Class " + suffix,
        Level:    1,
    }
    must(tx.Create(&class).Error == nil, "create smoke class")

    section := models.Section{
        SchoolID: school.ID,
        ClassID:  class.ID,
        Name:      "Smoke",
    }
    must(tx.Create(&section).Error == nil, "create smoke section")

    enrollment := models.StudentEnrollment{
        SchoolID:         school.ID,
        StudentID:        student.ID,
        AcademicSessionID: session.ID,
        ClassID:          class.ID,
        SectionID:        section.ID,
        Status:            models.EnrollmentStatusActive,
        EnrolledAt:        time.Now().UTC(),
    }
    must(tx.Create(&enrollment).Error == nil, "create smoke enrollment")

    fee := models.FeeItem{
        SchoolID: school.ID,
        TermID:   term.ID,
        Name:     "Smoke Tuition",
        Amount:   50000,
        Active:   true,
    }
    must(tx.Create(&fee).Error == nil, "create smoke fee")

    return school.ID, term, enrollment, models.Invoice{}, fee
}

func must(condition bool, format string, args ...interface{}) {
    if !condition {
        log.Fatalf("FINANCE_SMOKE_FAIL: "+format, args...)
    }
}
