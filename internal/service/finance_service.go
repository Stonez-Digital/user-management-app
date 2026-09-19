package service

import (
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
    ErrFeeItemNotFound = errors.New("fee item not found")
    ErrInvoiceNotFound = errors.New("invoice not found")
    ErrPaymentInvalid = errors.New("invalid payment")
    ErrPaymentDuplicate = errors.New("payment reference already exists")
    ErrFinanceInvalid = errors.New("invalid finance record")
)

type FinanceService struct { fees repository.FeeItemRepository; invoices repository.InvoiceRepository; payments repository.PaymentRepository; db *gorm.DB }
func NewFinanceService(f repository.FeeItemRepository,i repository.InvoiceRepository,p repository.PaymentRepository,db *gorm.DB)*FinanceService{return &FinanceService{fees:f,invoices:i,payments:p,db:db}}
func(s *FinanceService)DB()*gorm.DB{return s.db}

func(s *FinanceService)CreateFeeItem(v models.FeeItem)(models.FeeItem,error){
    v.Name=strings.TrimSpace(v.Name); if v.Name==""||v.Amount<=0{return v,ErrFinanceInvalid}
    var term models.Term;if e:=s.db.First(&term,"id=?",v.TermID).Error;e!=nil{return v,ErrFinanceInvalid}
    return s.fees.Create(v)
}
func(s *FinanceService)ListFeeItems()([]models.FeeItem,error){return s.fees.List()}
func(s *FinanceService)GetFeeItem(id uuid.UUID)(models.FeeItem,error){v,e:=s.fees.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrFeeItemNotFound};return v,e}
func(s *FinanceService)UpdateFeeItem(v models.FeeItem)error{if _,e:=s.GetFeeItem(v.ID);e!=nil{return e};if v.Amount<=0||strings.TrimSpace(v.Name)==""{return ErrFinanceInvalid};return s.fees.Update(v)}
func(s *FinanceService)DeleteFeeItem(id uuid.UUID)error{if _,e:=s.GetFeeItem(id);e!=nil{return e};return s.fees.Delete(id)}

type InvoiceLine struct{FeeItemID *uuid.UUID `json:"fee_item_id,omitempty"`;Name string `json:"name"`;Amount float64 `json:"amount"`}
type CreateInvoiceInput struct{EnrollmentID uuid.UUID;TermID uuid.UUID;DueDate *time.Time;Items []InvoiceLine}

func(s *FinanceService)CreateInvoice(in CreateInvoiceInput)(models.Invoice,[]models.InvoiceItem,error){
    var e models.StudentEnrollment;if err:=s.db.First(&e,"id=?",in.EnrollmentID).Error;err!=nil{return models.Invoice{},nil,ErrFinanceInvalid}
    var t models.Term;if err:=s.db.First(&t,"id=?",in.TermID).Error;err!=nil||t.AcademicSessionID!=e.AcademicSessionID{return models.Invoice{},nil,ErrFinanceInvalid}
    if len(in.Items)==0{return models.Invoice{},nil,ErrFinanceInvalid}
    total:=0.0; items:=make([]models.InvoiceItem,0,len(in.Items))
    for _,line:=range in.Items{if strings.TrimSpace(line.Name)==""||line.Amount<=0{return models.Invoice{},nil,ErrFinanceInvalid};total+=line.Amount;items=append(items,models.InvoiceItem{FeeItemID:line.FeeItemID,Name:strings.TrimSpace(line.Name),Amount:line.Amount})}
    invoice:=models.Invoice{EnrollmentID:in.EnrollmentID,TermID:in.TermID,InvoiceNumber:fmt.Sprintf("INV-%d",time.Now().UnixNano()),TotalAmount:total,Balance:total,Status:models.InvoiceStatusOpen,DueDate:in.DueDate}
    invoice,err:=s.invoices.Create(invoice);if err!=nil{return invoice,nil,err}
    for i:=range items{items[i].InvoiceID=invoice.ID;if err:=s.invoices.AddItem(items[i]);err!=nil{return invoice,nil,err}}
    return invoice,items,nil
}

func(s *FinanceService)ListInvoices()([]models.Invoice,error){return s.invoices.List()}
func(s *FinanceService)GetInvoice(id uuid.UUID)(models.Invoice,[]models.InvoiceItem,error){v,e:=s.invoices.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,nil,ErrInvoiceNotFound};if e!=nil{return v,nil,e};items,e:=s.invoices.Items(id);return v,items,e}

func(s *FinanceService)RecordPayment(invoiceID uuid.UUID,amount float64,provider,reference,status string)(models.Payment,error){
    if amount<=0||strings.TrimSpace(reference)==""{return models.Payment{},ErrPaymentInvalid}
    inv,_,e:=s.GetInvoice(invoiceID);if e!=nil{return models.Payment{},e}
    if status==""{status=models.PaymentStatusSuccessful};if status!=models.PaymentStatusSuccessful&&status!=models.PaymentStatusPending&&status!=models.PaymentStatusFailed{return models.Payment{},ErrPaymentInvalid}
    if amount>inv.Balance&&status==models.PaymentStatusSuccessful{return models.Payment{},ErrPaymentInvalid}
    if _,e=s.payments.FindByReference(reference);e==nil{return models.Payment{},ErrPaymentDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return models.Payment{},e}
    p:=models.Payment{InvoiceID:invoiceID,Amount:amount,Provider:strings.TrimSpace(provider),Reference:strings.TrimSpace(reference),Status:status};if p.Provider==""{p.Provider="manual"};if status==models.PaymentStatusSuccessful{now:=time.Now();p.PaidAt=&now}
    p,e=s.payments.Create(p);if e!=nil{return p,e}
    if status==models.PaymentStatusSuccessful{inv.PaidAmount+=amount;inv.Balance=inv.TotalAmount-inv.PaidAmount;if inv.Balance<=0{inv.Balance=0;inv.Status=models.InvoiceStatusPaid}else{inv.Status=models.InvoiceStatusPartiallyPaid};if e=s.invoices.Update(inv);e!=nil{return p,e}}
    return p,nil
}
func(s *FinanceService)ListPayments()([]models.Payment,error){return s.payments.List()}
