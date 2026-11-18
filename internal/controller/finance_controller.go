package controller

import (
    "encoding/json"
    "errors"
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/audit"
    "github.com/onoja217/users-management-app/internal/httpx"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/middleware"
    "github.com/onoja217/users-management-app/internal/service"
)

type FinanceController struct{service *service.FinanceService}
func NewFinanceController(s *service.FinanceService)*FinanceController{return &FinanceController{service:s}}

func (ctrl *FinanceController) requestService(c *gin.Context) (*service.FinanceService, bool) {
    tx, ok := middleware.TenantDBFromContext(c)
    if !ok {
        httpx.Error(c, http.StatusInternalServerError, "tenant_transaction_required", "tenant transaction is required for this operation")
        return nil, false
    }
    return ctrl.service.WithDB(tx), true
}

type feeRequest struct{Name string `json:"name" binding:"required,max=150"`;Description string `json:"description" binding:"max=500"`;Amount float64 `json:"amount" binding:"gt=0"`;Active *bool `json:"active"`}
type invoiceLineRequest struct{FeeItemID *uuid.UUID `json:"fee_item_id"`;Description string `json:"description" binding:"required,max=250"`;Quantity float64 `json:"quantity" binding:"gt=0"`;UnitAmount float64 `json:"unit_amount" binding:"gt=0"`}
type invoiceRequest struct{StudentEnrollmentID uuid.UUID `json:"student_enrollment_id" binding:"required"`;TermID uuid.UUID `json:"term_id" binding:"required"`;InvoiceNumber string `json:"invoice_number" binding:"max=50"`;DueDate string `json:"due_date" binding:"required"`;Lines []invoiceLineRequest `json:"lines" binding:"required,min=1"`}
type paymentRequest struct{Amount float64 `json:"amount" binding:"gt=0"`;Provider string `json:"provider" binding:"required,max=50"`;Reference string `json:"reference" binding:"required,max=150"`;Status string `json:"status"`;ReceiptNumber string `json:"receipt_number"`;PaidAt string `json:"paid_at"`;Metadata map[string]interface{} `json:"metadata"`}

func recordFinanceAudit(c *gin.Context, svc *service.FinanceService, action, resource string, id *uuid.UUID) bool {
    actor, err := uuid.Parse(c.GetString("user_id"))
    if err != nil {
        httpx.Error(c, http.StatusUnauthorized, "invalid_user_context", "invalid authenticated user")
        return false
    }
    if err := audit.Record(svc.DB(), c, &actor, action, resource, id, nil); err != nil {
        httpx.Error(c, http.StatusInternalServerError, "finance_audit_failed", "failed to record finance activity")
        return false
    }
    return true
}

func financeError(c *gin.Context,e error){switch{case errors.Is(e,service.ErrFeeNotFound):httpx.Error(c,404,"fee_not_found","fee item not found");case errors.Is(e,service.ErrFeeDuplicate):httpx.Error(c,409,"fee_duplicate","fee item already exists for this term");case errors.Is(e,service.ErrFeeInvalid):httpx.Error(c,400,"invalid_fee","invalid fee item");case errors.Is(e,service.ErrInvoiceNotFound):httpx.Error(c,404,"invoice_not_found","invoice not found");case errors.Is(e,service.ErrInvoiceEnrollmentMissing):httpx.Error(c,404,"enrollment_not_found","student enrollment not found");case errors.Is(e,service.ErrInvoiceTermMissing):httpx.Error(c,404,"term_not_found","invoice term not found");case errors.Is(e,service.ErrInvoiceInvalid):httpx.Error(c,400,"invalid_invoice","invalid invoice");case errors.Is(e,service.ErrPaymentDuplicate):httpx.Error(c,409,"payment_duplicate","payment reference already exists");case errors.Is(e,service.ErrPaymentExceedsBalance):httpx.Error(c,400,"payment_exceeds_balance","payment exceeds invoice balance");case errors.Is(e,service.ErrPaymentInvalid):httpx.Error(c,400,"invalid_payment","invalid payment");default:httpx.Error(c,500,"finance_operation_failed","finance operation failed")}}

func(ctrl *FinanceController)ListFees(c *gin.Context){svc,ok:=ctrl.requestService(c);if !ok{return};schoolID,ok:=requireSchoolID(c);if !ok{return};sessionID,termID,ok:=parseAcademicQuery(c);if !ok{return};v,e:=svc.ListFees(schoolID,sessionID,termID);if e!=nil{financeError(c,e);return};c.JSON(http.StatusOK,v)}
func(ctrl *FinanceController)GetFee(c *gin.Context){svc,ok:=ctrl.requestService(c);if !ok{return};schoolID,ok:=requireSchoolID(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_fee_id","invalid fee id");return};v,e:=svc.GetFee(schoolID,id);if e!=nil{financeError(c,e);return};c.JSON(http.StatusOK,v)}
func(ctrl *FinanceController)CreateFee(c *gin.Context){svc,ok:=ctrl.requestService(c);if !ok{return};schoolID,ok:=requireSchoolID(c);if !ok{return};tid,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_term_id","invalid term id");return};var r feeRequest;if e=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};active:=true;if r.Active!=nil{active=*r.Active};v,e:=svc.CreateFee(schoolID,models.FeeItem{TermID:tid,Name:r.Name,Description:r.Description,Amount:r.Amount,Active:active});if e!=nil{financeError(c,e);return};if !recordFinanceAudit(c,svc,"fee_item.create","fee_item",&v.ID){return};c.JSON(http.StatusCreated,v)}
func(ctrl *FinanceController)UpdateFee(c *gin.Context){svc,ok:=ctrl.requestService(c);if !ok{return};schoolID,ok:=requireSchoolID(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_fee_id","invalid fee id");return};var r feeRequest;if e=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};v,e:=svc.GetFee(schoolID,id);if e!=nil{financeError(c,e);return};v.Name=r.Name;v.Description=r.Description;v.Amount=r.Amount;if r.Active!=nil{v.Active=*r.Active};if e=svc.UpdateFee(schoolID,v);e!=nil{financeError(c,e);return};if !recordFinanceAudit(c,svc,"fee_item.update","fee_item",&v.ID){return};c.JSON(http.StatusOK,v)}
func(ctrl *FinanceController)DeleteFee(c *gin.Context){svc,ok:=ctrl.requestService(c);if !ok{return};schoolID,ok:=requireSchoolID(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_fee_id","invalid fee id");return};if e=svc.DeleteFee(schoolID,id);e!=nil{financeError(c,e);return};if !recordFinanceAudit(c,svc,"fee_item.delete","fee_item",&id){return};c.JSON(http.StatusOK,gin.H{"message":"fee item deleted"})}
func decorateInvoiceStudent(v *models.Invoice){v.StudentName=v.StudentEnrollment.Student.User.Name;v.StudentClassName=v.StudentEnrollment.SchoolClass.Name}
func(ctrl *FinanceController)ListInvoices(c *gin.Context){svc,ok:=ctrl.requestService(c);if !ok{return};schoolID,ok:=requireSchoolID(c);if !ok{return};sessionID,termID,ok:=parseAcademicQuery(c);if !ok{return};v,e:=svc.ListInvoices(schoolID,sessionID,termID);if e!=nil{financeError(c,e);return};for i:=range v{decorateInvoiceStudent(&v[i])};c.JSON(http.StatusOK,v)}
func(ctrl *FinanceController)GetInvoice(c *gin.Context){svc,ok:=ctrl.requestService(c);if !ok{return};schoolID,ok:=requireSchoolID(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_invoice_id","invalid invoice id");return};v,e:=svc.GetInvoice(schoolID,id);if e!=nil{financeError(c,e);return};decorateInvoiceStudent(&v);c.JSON(http.StatusOK,v)}
func(ctrl *FinanceController)CreateInvoice(c *gin.Context){svc,ok:=ctrl.requestService(c);if !ok{return};schoolID,ok:=requireSchoolID(c);if !ok{return};var r invoiceRequest;if e:=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};due,e:=time.Parse("2006-01-02",r.DueDate);if e!=nil{httpx.Error(c,400,"invalid_due_date","due_date must use YYYY-MM-DD format");return};lines:=make([]service.InvoiceLineInput,0,len(r.Lines));for _,l:=range r.Lines{lines=append(lines,service.InvoiceLineInput{FeeItemID:l.FeeItemID,Description:l.Description,Quantity:l.Quantity,UnitAmount:l.UnitAmount})};v,e:=svc.CreateInvoice(schoolID,models.Invoice{StudentEnrollmentID:r.StudentEnrollmentID,TermID:r.TermID,InvoiceNumber:r.InvoiceNumber,DueDate:due},lines);if e!=nil{financeError(c,e);return};if !recordFinanceAudit(c,svc,"invoice.create","invoice",&v.ID){return};c.JSON(http.StatusCreated,v)}
func(ctrl *FinanceController)ListPayments(c *gin.Context){svc,ok:=ctrl.requestService(c);if !ok{return};schoolID,ok:=requireSchoolID(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_invoice_id","invalid invoice id");return};v,e:=svc.ListPayments(schoolID,id);if e!=nil{financeError(c,e);return};c.JSON(http.StatusOK,v)}
func(ctrl *FinanceController)CreatePayment(c *gin.Context){svc,ok:=ctrl.requestService(c);if !ok{return};schoolID,ok:=requireSchoolID(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_invoice_id","invalid invoice id");return};var r paymentRequest;if e=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};status:=r.Status;if status==""{status=models.PaymentStatusPending};if status!=models.PaymentStatusPending&&status!=models.PaymentStatusSucceeded&&status!=models.PaymentStatusFailed&&status!=models.PaymentStatusRefunded{httpx.Error(c,400,"invalid_payment_status","invalid payment status");return};var paidAt *time.Time;if r.PaidAt!=""{d,e:=time.Parse(time.RFC3339,r.PaidAt);if e!=nil{httpx.Error(c,400,"invalid_paid_at","paid_at must be RFC3339");return};paidAt=&d};meta:="";if r.Metadata!=nil{b,e:=json.Marshal(r.Metadata);if e!=nil{httpx.Error(c,400,"invalid_metadata","invalid metadata");return};meta=string(b)};v,e:=svc.CreatePayment(schoolID,models.Payment{InvoiceID:id,Amount:r.Amount,Provider:r.Provider,Reference:r.Reference,Status:status,ReceiptNumber:r.ReceiptNumber,PaidAt:paidAt,Metadata:meta});if e!=nil{financeError(c,e);return};if !recordFinanceAudit(c,svc,"payment.create","payment",&v.ID){return};c.JSON(http.StatusCreated,v)}
