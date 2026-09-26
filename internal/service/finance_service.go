package service

import (
    "encoding/json"
    "errors"
    "fmt"
    "strings"
    "time"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/gorm"
    "gorm.io/gorm/clause"
)

var (
    ErrFeeNotFound=errors.New("fee item not found")
    ErrFeeInvalid=errors.New("invalid fee item")
    ErrFeeDuplicate=errors.New("fee item already exists for this term")
    ErrInvoiceNotFound=errors.New("invoice not found")
    ErrInvoiceInvalid=errors.New("invalid invoice")
    ErrInvoiceTermMissing=errors.New("invoice term not found")
    ErrInvoiceEnrollmentMissing=errors.New("invoice enrollment not found")
    ErrPaymentInvalid=errors.New("invalid payment")
    ErrPaymentDuplicate=errors.New("payment reference already exists")
    ErrPaymentExceedsBalance=errors.New("payment exceeds invoice balance")
)

type PaymentProvider interface { Name() string; Verify(string)(PaymentVerification,error) }
type PaymentVerification struct { Reference string; Amount float64; Status string; PaidAt time.Time; Metadata map[string]interface{} }

type FinanceService struct { fees repository.FeeItemRepository; invoices repository.InvoiceRepository; payments repository.PaymentRepository; db *gorm.DB }
func NewFinanceService(f repository.FeeItemRepository,i repository.InvoiceRepository,p repository.PaymentRepository,db *gorm.DB)*FinanceService{return &FinanceService{fees:f,invoices:i,payments:p,db:db}}
func(s *FinanceService)DB()*gorm.DB{return s.db}

func(s *FinanceService)CreateFee(schoolID uuid.UUID,v models.FeeItem)(models.FeeItem,error){
    if schoolID==uuid.Nil||v.TermID==uuid.Nil||strings.TrimSpace(v.Name)==""||v.Amount<=0{return v,ErrFeeInvalid}
    var term models.Term;if e:=s.db.Where("id = ? AND school_id = ?",v.TermID,schoolID).First(&term).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrInvoiceTermMissing};return v,e}
    v.SchoolID=schoolID;v.Name=strings.TrimSpace(v.Name);var x models.FeeItem
    e:=s.db.Where("school_id=? AND term_id=? AND lower(name)=lower(?)",schoolID,v.TermID,v.Name).First(&x).Error
    if e==nil{return v,ErrFeeDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return v,e}
    return s.fees.Create(schoolID,v)
}
func(s *FinanceService)ListFees(schoolID,sessionID,termID uuid.UUID)([]models.FeeItem,error){var term models.Term;if e:=s.db.Where("id=? AND school_id=? AND academic_session_id=?",termID,schoolID,sessionID).First(&term).Error;e!=nil{return nil,ErrInvoiceTermMissing};return s.fees.List(schoolID,&termID)}
func(s *FinanceService)GetFee(schoolID,id uuid.UUID)(models.FeeItem,error){v,e:=s.fees.Get(schoolID,id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrFeeNotFound};return v,e}
func(s *FinanceService)UpdateFee(schoolID uuid.UUID,v models.FeeItem)error{
    current,e:=s.GetFee(schoolID,v.ID);if e!=nil{return e};if v.TermID==uuid.Nil||strings.TrimSpace(v.Name)==""||v.Amount<=0{return ErrFeeInvalid}
    v.TermID=current.TermID;v.Name=strings.TrimSpace(v.Name);var x models.FeeItem
    e=s.db.Where("school_id=? AND term_id=? AND lower(name)=lower(?) AND id<>?",schoolID,v.TermID,v.Name,v.ID).First(&x).Error
    if e==nil{return ErrFeeDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return e};return s.fees.Update(schoolID,v)
}
func(s *FinanceService)DeleteFee(schoolID,id uuid.UUID)error{if _,e:=s.GetFee(schoolID,id);e!=nil{return e};return s.fees.Delete(schoolID,id)}

type InvoiceLineInput struct { FeeItemID *uuid.UUID; Description string; Quantity float64; UnitAmount float64 }

func(s *FinanceService)CreateInvoice(schoolID uuid.UUID,v models.Invoice,lines []InvoiceLineInput)(models.Invoice,error){
    if schoolID==uuid.Nil||v.StudentEnrollmentID==uuid.Nil||v.TermID==uuid.Nil||v.DueDate.IsZero()||len(lines)==0{return v,ErrInvoiceInvalid}
    var enrollment models.StudentEnrollment
    if e:=s.db.Where("id = ? AND school_id = ?",v.StudentEnrollmentID,schoolID).First(&enrollment).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrInvoiceEnrollmentMissing};return v,e}
    if enrollment.Status!=models.EnrollmentStatusActive{return v,ErrInvoiceInvalid}
    var term models.Term
    if e:=s.db.Where("id = ? AND school_id = ? AND academic_session_id = ?",v.TermID,schoolID,enrollment.AcademicSessionID).First(&term).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrInvoiceTermMissing};return v,e}
    if v.DueDate.Before(term.StartDate)||v.DueDate.After(term.EndDate){return v,ErrInvoiceInvalid}
    if v.InvoiceNumber==""{v.InvoiceNumber=fmt.Sprintf("INV-%s-%s",time.Now().UTC().Format("20060102"),uuid.NewString()[:8])}
    v.SchoolID=schoolID;v.Status=models.InvoiceStatusIssued;v.PaidAmount=0
    var total float64
    for _,in:=range lines{
        if in.Quantity<=0||in.UnitAmount<=0||strings.TrimSpace(in.Description)==""{return v,ErrInvoiceInvalid}
        if in.FeeItemID!=nil{var fee models.FeeItem;if e:=s.db.Where("id = ? AND school_id = ? AND term_id = ? AND active = ?",*in.FeeItemID,schoolID,v.TermID,true).First(&fee).Error;e!=nil{return v,ErrInvoiceInvalid}}
        total+=in.Quantity*in.UnitAmount
    }
    if total<=0{return v,ErrInvoiceInvalid};v.TotalAmount=total;v.Balance=total
    err:=s.db.Transaction(func(tx *gorm.DB)error{
        if e:=tx.Create(&v).Error;e!=nil{return e}
        for _,in:=range lines{l:=models.InvoiceLine{SchoolID:schoolID,InvoiceID:v.ID,FeeItemID:in.FeeItemID,Description:strings.TrimSpace(in.Description),Quantity:in.Quantity,UnitAmount:in.UnitAmount,Amount:in.Quantity*in.UnitAmount};if e:=tx.Create(&l).Error;e!=nil{return e}}
        return nil
    })
    if err!=nil{return v,err};return s.invoices.Get(schoolID,v.ID)
}
func(s *FinanceService)ListInvoices(schoolID,sessionID,termID uuid.UUID)([]models.Invoice,error){rows,e:=s.invoices.List(schoolID);if e!=nil{return nil,e};out:=make([]models.Invoice,0,len(rows));for _,r:=range rows{if r.TermID==termID&&r.Term.AcademicSessionID==sessionID{out=append(out,r)}};return out,nil}
func(s *FinanceService)GetInvoice(schoolID,id uuid.UUID)(models.Invoice,error){v,e:=s.invoices.Get(schoolID,id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrInvoiceNotFound};return v,e}

func(s *FinanceService)CreatePayment(schoolID uuid.UUID,v models.Payment)(models.Payment,error){
    if schoolID==uuid.Nil||v.InvoiceID==uuid.Nil||v.Amount<=0||strings.TrimSpace(v.Provider)==""||strings.TrimSpace(v.Reference)==""{return v,ErrPaymentInvalid}
    if v.Status==""{v.Status=models.PaymentStatusPending}
    if v.Status!=models.PaymentStatusPending&&v.Status!=models.PaymentStatusSucceeded&&v.Status!=models.PaymentStatusFailed&&v.Status!=models.PaymentStatusRefunded{return v,ErrPaymentInvalid}
    if v.Metadata!=""{var raw interface{};if json.Unmarshal([]byte(v.Metadata),&raw)!=nil{return v,ErrPaymentInvalid}}
    if v.ReceiptNumber==""{v.ReceiptNumber=fmt.Sprintf("RCT-%s-%s",time.Now().UTC().Format("20060102"),uuid.NewString()[:8])};v.SchoolID=schoolID
    err:=s.db.Transaction(func(tx *gorm.DB)error{
        var invoice models.Invoice
        if e:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).Where("id = ? AND school_id = ?",v.InvoiceID,schoolID).First(&invoice).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return ErrInvoiceNotFound};return e}
        if invoice.Status==models.InvoiceStatusCancelled||invoice.Balance<=0{return ErrPaymentInvalid}
        var existing models.Payment
        if e:=tx.Where("reference = ?",v.Reference).First(&existing).Error;e==nil{return ErrPaymentDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return e}
        if v.Status==models.PaymentStatusSucceeded{if v.Amount>invoice.Balance+0.000001{return ErrPaymentExceedsBalance};if v.PaidAt==nil{now:=time.Now().UTC();v.PaidAt=&now}}
        if e:=tx.Create(&v).Error;e!=nil{return e}
        if v.Status==models.PaymentStatusSucceeded{
            var sum float64
            if e:=tx.Model(&models.Payment{}).Where("school_id=? AND invoice_id=? AND status=?",schoolID,invoice.ID,models.PaymentStatusSucceeded).Select("COALESCE(SUM(amount),0)").Scan(&sum).Error;e!=nil{return e}
            invoice.PaidAmount=sum;invoice.Balance=invoice.TotalAmount-sum;if invoice.Balance<0{invoice.Balance=0}
            if invoice.Balance==0{invoice.Status=models.InvoiceStatusPaid}else{invoice.Status=models.InvoiceStatusPartiallyPaid}
            if e:=tx.Model(&models.Invoice{}).Where("id = ? AND school_id = ?",invoice.ID,schoolID).Updates(map[string]interface{}{"paid_amount":invoice.PaidAmount,"balance":invoice.Balance,"status":invoice.Status}).Error;e!=nil{return e}
        };return nil
    })
    if err!=nil{return v,err};return v,nil
}
func(s *FinanceService)ListPayments(schoolID,invoiceID uuid.UUID)([]models.Payment,error){if _,e:=s.GetInvoice(schoolID,invoiceID);e!=nil{return nil,e};return s.payments.ListByInvoice(schoolID,invoiceID)}
