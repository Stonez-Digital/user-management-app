package service

import (
    "testing"
    "time"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func financeTestDB(t *testing.T)*gorm.DB{
    t.Helper()
    db,e:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if e!=nil{t.Fatal(e)}
    if e=db.AutoMigrate(&models.User{},&models.Student{},&models.AcademicSession{},&models.Term{},&models.SchoolClass{},&models.Section{},&models.StudentEnrollment{},&models.FeeItem{},&models.Invoice{},&models.InvoiceLine{},&models.Payment{});e!=nil{t.Fatal(e)}
    return db
}

func TestFinanceInvoiceAndPaymentWorkflow(t *testing.T){
    db:=financeTestDB(t)
    studentUser:=models.User{Name:"Finance Student",Email:"finance-student@example.com",Role:"student",Active:true};if e:=db.Create(&studentUser).Error;e!=nil{t.Fatal(e)}
    student:=models.Student{UserID:studentUser.ID,AdmissionNumber:"FIN-001"};if e:=db.Create(&student).Error;e!=nil{t.Fatal(e)}
    session:=models.AcademicSession{Name:"2026/2027",StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,7,31,0,0,0,0,time.UTC)};if e:=db.Create(&session).Error;e!=nil{t.Fatal(e)}
    term:=models.Term{AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2026,12,15,0,0,0,0,time.UTC)};if e:=db.Create(&term).Error;e!=nil{t.Fatal(e)}
    class:=models.SchoolClass{Name:"JSS 1",Level:1};if e:=db.Create(&class).Error;e!=nil{t.Fatal(e)}
    section:=models.Section{Name:"A",ClassID:class.ID};if e:=db.Create(&section).Error;e!=nil{t.Fatal(e)}
    enrollment:=models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:section.ID,Status:models.EnrollmentStatusActive,EnrolledAt:time.Now().UTC()};if e:=db.Create(&enrollment).Error;e!=nil{t.Fatal(e)}
    svc:=NewFinanceService(repository.NewFeeItemRepository(db),repository.NewInvoiceRepository(db),repository.NewPaymentRepository(db),db)
    fee,e:=svc.CreateFee(models.FeeItem{TermID:term.ID,Name:"Tuition",Amount:50000,Active:true});if e!=nil{t.Fatal(e)}
    invoice,e:=svc.CreateInvoice(models.Invoice{StudentEnrollmentID:enrollment.ID,TermID:term.ID,DueDate:time.Date(2026,11,30,0,0,0,0,time.UTC)},[]InvoiceLineInput{{FeeItemID:&fee.ID,Description:"Tuition",Quantity:1,UnitAmount:50000}});if e!=nil{t.Fatal(e)}
    if invoice.TotalAmount!=50000||invoice.Balance!=50000{t.Fatalf("unexpected invoice totals: total=%v balance=%v",invoice.TotalAmount,invoice.Balance)}
    _,e=svc.CreatePayment(models.Payment{InvoiceID:invoice.ID,Amount:20000,Provider:"manual",Reference:"PAY-001",Status:models.PaymentStatusSucceeded});if e!=nil{t.Fatal(e)}
    updated,e:=svc.GetInvoice(invoice.ID);if e!=nil{t.Fatal(e)}
    if updated.PaidAmount!=20000||updated.Balance!=30000||updated.Status!=models.InvoiceStatusPartiallyPaid{t.Fatalf("unexpected partial payment state: paid=%v balance=%v status=%s",updated.PaidAmount,updated.Balance,updated.Status)}
    _,e=svc.CreatePayment(models.Payment{InvoiceID:invoice.ID,Amount:20000,Provider:"manual",Reference:"PAY-001",Status:models.PaymentStatusSucceeded});if e!=ErrPaymentDuplicate{t.Fatalf("expected duplicate reference, got %v",e)}
    _,e=svc.CreatePayment(models.Payment{InvoiceID:invoice.ID,Amount:30001,Provider:"manual",Reference:"PAY-002",Status:models.PaymentStatusSucceeded});if e!=ErrPaymentExceedsBalance{t.Fatalf("expected balance validation, got %v",e)}
    _,e=svc.CreatePayment(models.Payment{InvoiceID:invoice.ID,Amount:30000,Provider:"manual",Reference:"PAY-003",Status:models.PaymentStatusSucceeded});if e!=nil{t.Fatal(e)}
    updated,e=svc.GetInvoice(invoice.ID);if e!=nil{t.Fatal(e)}
    if updated.PaidAmount!=50000||updated.Balance!=0||updated.Status!=models.InvoiceStatusPaid{t.Fatalf("unexpected paid state: paid=%v balance=%v status=%s",updated.PaidAmount,updated.Balance,updated.Status)}
}
