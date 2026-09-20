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

func(s *FinanceService)CreateFee(v models.FeeItem)(models.FeeItem,error){
    if v.TermID==uuid.Nil||strings.TrimSpace(v.Name)==""||v.Amount<=0{return v,ErrFeeInvalid}
    var term models.Term;if e:=s.db.First(&term,"id = ?",v.TermID).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrInvoiceTermMissing};return v,e}
    v.Name=strings.TrimSpace(v.Name);var x models.FeeItem
    e:=s.db.Where("term_id=? AND lower(name)=lower(?)",v.TermID,v.Name).First(&x).Error
    if e==nil{return v,ErrFeeDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return v,e}
    return s.fees.Create(v)
}
func(s *FinanceService)ListFees(termID *uuid.UUID)([]models.FeeItem,error){return s.fees.List(termID)}
func(s *FinanceService)GetFee(id uuid.UUID)(models.FeeItem,error){v,e:=s.fees.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrFeeNotFound};return v,e}
func(s *FinanceService)UpdateFee(v models.FeeItem)error{if _,e:=s.GetFee(v.ID);e!=nil{return e};if v.TermID==uuid.Nil||strings.TrimSpace(v.Name)==""||v.Amount<=0{return ErrFeeInvalid};v.Name=strings.TrimSpace(v.Name);var x models.FeeItem;e:=s.db.Where("term_id=? AND lower(name)=lower(?) AND id<>?",v.TermID,v.Name,v.ID).First(&x).Error;if e==nil{return ErrFeeDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return e};return s.fees.Update(v)}
func(s *FinanceService)DeleteFee(id uuid.UUID)error{if _,e:=s.GetFee(id);e!=nil{return e};return s.fees.Delete(id)}

type InvoiceLineInput struct { FeeItemID *uuid.UUID; Description string; Quantity float64; UnitAmount float64 }

func(s *FinanceService)CreateInvoice(v models.Invoice,lines []InvoiceLineInput)(models.Invoice,error){
    if v.StudentEnrollmentID==uuid.Nil||v.TermID==uuid.Nil||v.DueDate.IsZero()||len(lines)==0{return v,ErrInvoiceInvalid}
    var enrollment models.StudentEnrollment
    if e:=s.db.First(&enrollment,"id = ?",v.StudentEnrollmentID).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrInvoiceEnrollmentMissing};return v,e}
    if enrollment.Status==models.EnrollmentStatusWithdrawn{return v,ErrInvoiceInvalid}
    var term models.Term
    if e:=s.db.First(&term,"id = ? AND academic_session_id = ?",v.TermID,enrollment.AcademicSessionID).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrInvoiceTermMissing};return v,e}
    if v.DueDate.Before(term.StartDate)||v.DueDate.After(term.EndDate){return v,ErrInvoiceInvalid}
    if v.InvoiceNumber==""{v.InvoiceNumber=fmt.Sprintf("INV-%s-%s",time.Now().UTC().Format("20060102"),uuid.NewString()[:8])}
    v.Status=models.InvoiceStatusIssued;v.PaidAmount=0
    var total float64
    for _,in:=range lines{
        if in.Quantity<=0||in.UnitAmount<=0||strings.TrimSpace(in.Description)==""{return v,ErrInvoiceInvalid}
        if in.FeeItemID!=nil{var fee models.FeeItem;if e:=s.db.First(&fee,"id = ? AND term_id = ? AND active = ?",*in.FeeItemID,v.TermID,true).Error;e!=nil{return v,ErrInvoiceInvalid}}
        total+=in.Quantity*in.UnitAmount
    }
    if total<=0{return v,ErrInvoiceInvalid};v.TotalAmount=total;v.Balance=total
    err:=s.db.Transaction(func(tx *gorm.DB)error{
        if e:=tx.Create(&v).Error;e!=nil{return e}
        for _,in:=range lines{l:=models.InvoiceLine{InvoiceID:v.ID,FeeItemID:in.FeeItemID,Description:strings.TrimSpace(in.Description),Quantity:in.Quantity,UnitAmount:in.UnitAmount,Amount:in.Quantity*in.UnitAmount};if e:=tx.Create(&l).Error;e!=nil{return e}}
        return nil
    })
    if err!=nil{return v,err};return s.invoices.Get(v.ID)
}
func(s *FinanceService)ListInvoices()([]models.Invoice,error){return s.invoices.List()}
func(s *FinanceService)GetInvoice(id uuid.UUID)(models.Invoice,error){v,e:=s.invoices.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrInvoiceNotFound};return v,e}

func(s *FinanceService)CreatePayment(v models.Payment)(models.Payment,error){
    if v.InvoiceID==uuid.Nil||v.Amount<=0||strings.TrimSpace(v.Provider)==""||strings.TrimSpace(v.Reference)==""{return v,ErrPaymentInvalid}
    invoice,e:=s.GetInvoice(v.InvoiceID);if e!=nil{return v,e}
    if invoice.Status==models.InvoiceStatusCancelled||invoice.Balance<=0{return v,ErrPaymentInvalid}
    existing,e:=s.payments.GetByReference(v.Reference);if e==nil{return v,ErrPaymentDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return v,e}
    if v.Status==""{v.Status=models.PaymentStatusPending}
    if v.Status==models.PaymentStatusSucceeded{
        if v.Amount>invoice.Balance+0.000001{return v,ErrPaymentExceedsBalance}
        if v.PaidAt==nil{now:=time.Now().UTC();v.PaidAt=&now}
    }
    if v.ReceiptNumber==""{v.ReceiptNumber=fmt.Sprintf("RCT-%s-%s",time.Now().UTC().Format("20060102"),uuid.NewString()[:8])}
    if v.Metadata!=""{var raw interface{};if json.Unmarshal([]byte(v.Metadata),&raw)!=nil{return v,ErrPaymentInvalid}}
    err=s.db.Transaction(func(tx *gorm.DB)error{
        if e:=tx.Create(&v).Error;e!=nil{return e}
        if v.Status==models.PaymentStatusSucceeded{
            var sum float64
            if e:=tx.Model(&models.Payment{}).Where("invoice_id=? AND status=?",invoice.ID,models.PaymentStatusSucceeded).Select("COALESCE(SUM(amount),0)").Scan(&sum).Error;e!=nil{return e}
            invoice.PaidAmount=sum;invoice.Balance=invoice.TotalAmount-sum;if invoice.Balance<0{invoice.Balance=0}
            if invoice.Balance==0{invoice.Status=models.InvoiceStatusPaid}else{invoice.Status=models.InvoiceStatusPartiallyPaid}
            if e:=tx.Model(&models.Invoice{}).Where("id = ?",invoice.ID).Updates(map[string]interface{}{"paid_amount":invoice.PaidAmount,"balance":invoice.Balance,"status":invoice.Status}).Error;e!=nil{return e}
        }
        return nil
    })
    if err!=nil{return v,err};return v,nil
}
func(s *FinanceService)ListPayments(invoiceID uuid.UUID)([]models.Payment,error){if _,e:=s.GetInvoice(invoiceID);e!=nil{return nil,e};return s.payments.ListByInvoice(invoiceID)}
