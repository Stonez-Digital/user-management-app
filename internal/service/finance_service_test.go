package service

import (
    "testing"
    "time"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func financeTestDB(t *testing.T)*gorm.DB{
    t.Helper()
    db,e:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if e!=nil{t.Fatal(e)}
    if e=db.AutoMigrate(&models.School{},&models.User{},&models.Student{},&models.AcademicSession{},&models.Term{},&models.SchoolClass{},&models.Section{},&models.StudentEnrollment{},&models.FeeItem{},&models.Invoice{},&models.InvoiceLine{},&models.Payment{});e!=nil{t.Fatal(e)}
    return db
}

func financeFixture(t *testing.T,db *gorm.DB,code string)(uuid.UUID,models.Term,models.StudentEnrollment){
    t.Helper()
    school:=models.School{Name:"School "+code,Code:code};if e:=db.Create(&school).Error;e!=nil{t.Fatal(e)}
    u:=models.User{Name:"Finance Student "+code,Email:"finance-"+code+"@example.com",Role:"student",Active:true,SchoolID:&school.ID};if e:=db.Create(&u).Error;e!=nil{t.Fatal(e)}
    st:=models.Student{SchoolID:school.ID,UserID:u.ID,AdmissionNumber:"FIN-"+code};if e:=db.Create(&st).Error;e!=nil{t.Fatal(e)}
    session:=models.AcademicSession{SchoolID:school.ID,Name:"2026/2027",StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,7,31,0,0,0,0,time.UTC)};if e:=db.Create(&session).Error;e!=nil{t.Fatal(e)}
    term:=models.Term{SchoolID:school.ID,AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2026,12,15,0,0,0,0,time.UTC)};if e:=db.Create(&term).Error;e!=nil{t.Fatal(e)}
    class:=models.SchoolClass{SchoolID:school.ID,Name:"JSS 1",Level:1};if e:=db.Create(&class).Error;e!=nil{t.Fatal(e)}
    section:=models.Section{SchoolID:school.ID,Name:"A",ClassID:class.ID};if e:=db.Create(&section).Error;e!=nil{t.Fatal(e)}
    enrollment:=models.StudentEnrollment{SchoolID:school.ID,StudentID:st.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:section.ID,Status:models.EnrollmentStatusActive,EnrolledAt:time.Now().UTC()};if e:=db.Create(&enrollment).Error;e!=nil{t.Fatal(e)}
    return school.ID,term,enrollment
}

func TestFinanceInvoiceAndPaymentWorkflow(t *testing.T){
    db:=financeTestDB(t)
    schoolID,term,enrollment:=financeFixture(t,db,"A")
    svc:=NewFinanceService(repository.NewFeeItemRepository(db),repository.NewInvoiceRepository(db),repository.NewPaymentRepository(db),db)
    fee,e:=svc.CreateFee(schoolID,models.FeeItem{TermID:term.ID,Name:"Tuition",Amount:50000,Active:true});if e!=nil{t.Fatal(e)}
    invoice,e:=svc.CreateInvoice(schoolID,models.Invoice{StudentEnrollmentID:enrollment.ID,TermID:term.ID,DueDate:time.Date(2026,11,30,0,0,0,0,time.UTC)},[]InvoiceLineInput{{FeeItemID:&fee.ID,Description:"Tuition",Quantity:1,UnitAmount:50000}});if e!=nil{t.Fatal(e)}
    if invoice.TotalAmount!=50000||invoice.Balance!=50000{t.Fatalf("unexpected invoice totals: total=%v balance=%v",invoice.TotalAmount,invoice.Balance)}
    _,e=svc.CreatePayment(schoolID,models.Payment{InvoiceID:invoice.ID,Amount:20000,Provider:"manual",Reference:"PAY-001",Status:models.PaymentStatusSucceeded});if e!=nil{t.Fatal(e)}
    updated,e:=svc.GetInvoice(schoolID,invoice.ID);if e!=nil{t.Fatal(e)}
    if updated.PaidAmount!=20000||updated.Balance!=30000||updated.Status!=models.InvoiceStatusPartiallyPaid{t.Fatalf("unexpected partial payment state: paid=%v balance=%v status=%s",updated.PaidAmount,updated.Balance,updated.Status)}
    _,e=svc.CreatePayment(schoolID,models.Payment{InvoiceID:invoice.ID,Amount:20000,Provider:"manual",Reference:"PAY-001",Status:models.PaymentStatusSucceeded});if e!=ErrPaymentDuplicate{t.Fatalf("expected duplicate reference, got %v",e)}
    _,e=svc.CreatePayment(schoolID,models.Payment{InvoiceID:invoice.ID,Amount:30001,Provider:"manual",Reference:"PAY-002",Status:models.PaymentStatusSucceeded});if e!=ErrPaymentExceedsBalance{t.Fatalf("expected balance validation, got %v",e)}
    _,e=svc.CreatePayment(schoolID,models.Payment{InvoiceID:invoice.ID,Amount:30000,Provider:"manual",Reference:"PAY-003",Status:models.PaymentStatusSucceeded});if e!=nil{t.Fatal(e)}
    updated,e=svc.GetInvoice(schoolID,invoice.ID);if e!=nil{t.Fatal(e)}
    if updated.PaidAmount!=50000||updated.Balance!=0||updated.Status!=models.InvoiceStatusPaid{t.Fatalf("unexpected paid state: paid=%v balance=%v status=%s",updated.PaidAmount,updated.Balance,updated.Status)}
}

func TestFinanceSchoolIsolation(t *testing.T){
    db:=financeTestDB(t)
    schoolA,termA,enrollmentA:=financeFixture(t,db,"A")
    schoolB,termB,enrollmentB:=financeFixture(t,db,"B")
    svc:=NewFinanceService(repository.NewFeeItemRepository(db),repository.NewInvoiceRepository(db),repository.NewPaymentRepository(db),db)
    feeA,e:=svc.CreateFee(schoolA,models.FeeItem{TermID:termA.ID,Name:"Tuition",Amount:50000,Active:true});if e!=nil{t.Fatal(e)}
    feeB,e:=svc.CreateFee(schoolB,models.FeeItem{TermID:termB.ID,Name:"Tuition",Amount:50000,Active:true});if e!=nil{t.Fatal(e)}
    if feeA.ID==feeB.ID{t.Fatal("expected distinct fee items")}
    if _,e:=svc.GetFee(schoolB,feeA.ID);e!=ErrFeeNotFound{t.Fatalf("expected cross-school fee read to fail, got %v",e)}
    if e:=svc.DeleteFee(schoolB,feeA.ID);e!=ErrFeeNotFound{t.Fatalf("expected cross-school fee delete to fail, got %v",e)}
    invA,e:=svc.CreateInvoice(schoolA,models.Invoice{StudentEnrollmentID:enrollmentA.ID,TermID:termA.ID,InvoiceNumber:"INV-A",DueDate:time.Date(2026,11,30,0,0,0,0,time.UTC)},[]InvoiceLineInput{{FeeItemID:&feeA.ID,Description:"Tuition",Quantity:1,UnitAmount:50000}});if e!=nil{t.Fatal(e)}
    if _,e:=svc.GetInvoice(schoolB,invA.ID);e!=ErrInvoiceNotFound{t.Fatalf("expected cross-school invoice read to fail, got %v",e)}
    if _,e:=svc.CreateInvoice(schoolB,models.Invoice{StudentEnrollmentID:enrollmentA.ID,TermID:termB.ID,DueDate:time.Date(2026,11,30,0,0,0,0,time.UTC)},[]InvoiceLineInput{{FeeItemID:&feeB.ID,Description:"Tuition",Quantity:1,UnitAmount:50000}});e!=ErrInvoiceEnrollmentMissing{t.Fatalf("expected cross-school enrollment rejection, got %v",e)}
    if _,e:=svc.CreateInvoice(schoolA,models.Invoice{StudentEnrollmentID:enrollmentA.ID,TermID:termB.ID,DueDate:time.Date(2026,11,30,0,0,0,0,time.UTC)},[]InvoiceLineInput{{FeeItemID:&feeA.ID,Description:"Tuition",Quantity:1,UnitAmount:50000}});e!=ErrInvoiceTermMissing{t.Fatalf("expected cross-school term rejection, got %v",e)}
    if _,e:=svc.ListPayments(schoolB,invA.ID);e!=ErrInvoiceNotFound{t.Fatalf("expected cross-school payment list to fail, got %v",e)}
    if _,e:=svc.CreatePayment(schoolB,models.Payment{InvoiceID:invA.ID,Amount:1000,Provider:"manual",Reference:"PAY-B",Status:models.PaymentStatusSucceeded});e!=ErrInvoiceNotFound{t.Fatalf("expected cross-school payment creation to fail, got %v",e)}
}
