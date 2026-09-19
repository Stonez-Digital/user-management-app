package controller

import (
    "errors"
    "net/http"
    "time"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/audit"
    "github.com/onoja217/users-management-app/internal/httpx"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/service"
)

type FinanceController struct{service *service.FinanceService}
func NewFinanceController(s *service.FinanceService)*FinanceController{return &FinanceController{service:s}}

func financeError(c *gin.Context,e error){switch{case errors.Is(e,service.ErrFeeItemNotFound):httpx.Error(c,404,"fee_item_not_found","fee item not found");case errors.Is(e,service.ErrInvoiceNotFound):httpx.Error(c,404,"invoice_not_found","invoice not found");case errors.Is(e,service.ErrPaymentDuplicate):httpx.Error(c,409,"payment_duplicate","payment reference already exists");case errors.Is(e,service.ErrFinanceInvalid),errors.Is(e,service.ErrPaymentInvalid):httpx.Error(c,400,"invalid_finance_record","invalid finance record");default:httpx.Error(c,500,"finance_operation_failed","finance operation failed")}}

type feeItemRequest struct{TermID uuid.UUID `json:"term_id" binding:"required"`;Name string `json:"name" binding:"required,max=150"`;Amount float64 `json:"amount" binding:"required"`;Active *bool `json:"active"`}
func(cn *FinanceController)ListFeeItems(c *gin.Context){v,e:=cn.service.ListFeeItems();if e!=nil{financeError(c,e);return};c.JSON(200,v)}
func(cn *FinanceController)CreateFeeItem(c *gin.Context){var r feeItemRequest;if e:=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};active:=true;if r.Active!=nil{active=*r.Active};v,e:=cn.service.CreateFeeItem(models.FeeItem{TermID:r.TermID,Name:r.Name,Amount:r.Amount,Active:active});if e!=nil{financeError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(cn.service.DB(),c,&actor,"fee_item.create","fee_item",&v.ID,nil);c.JSON(http.StatusCreated,v)}
func(cn *FinanceController)GetFeeItem(c *gin.Context){id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_fee_item_id","invalid fee item id");return};v,e:=cn.service.GetFeeItem(id);if e!=nil{financeError(c,e);return};c.JSON(200,v)}
func(cn *FinanceController)UpdateFeeItem(c *gin.Context){id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_fee_item_id","invalid fee item id");return};var r feeItemRequest;if e=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};active:=true;if r.Active!=nil{active=*r.Active};v:=models.FeeItem{ID:id,TermID:r.TermID,Name:r.Name,Amount:r.Amount,Active:active};if e=cn.service.UpdateFeeItem(v);e!=nil{financeError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(cn.service.DB(),c,&actor,"fee_item.update","fee_item",&id,nil);c.JSON(200,v)}
func(cn *FinanceController)DeleteFeeItem(c *gin.Context){id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_fee_item_id","invalid fee item id");return};if e=cn.service.DeleteFeeItem(id);e!=nil{financeError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(cn.service.DB(),c,&actor,"fee_item.delete","fee_item",&id,nil);c.JSON(200,gin.H{"message":"fee item deleted"})}

type invoiceLineRequest struct{FeeItemID *uuid.UUID `json:"fee_item_id"`;Name string `json:"name" binding:"required,max=150"`;Amount float64 `json:"amount" binding:"required"`}
type invoiceRequest struct{EnrollmentID uuid.UUID `json:"enrollment_id" binding:"required"`;TermID uuid.UUID `json:"term_id" binding:"required"`;DueDate string `json:"due_date"`;Items []invoiceLineRequest `json:"items" binding:"required,min=1"`}
func(cn *FinanceController)CreateInvoice(c *gin.Context){var r invoiceRequest;if e:=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};var due *time.Time;if r.DueDate!=""{d,e:=time.Parse("2006-01-02",r.DueDate);if e!=nil{httpx.Error(c,400,"invalid_due_date","due_date must be YYYY-MM-DD");return};due=&d};items:=make([]service.InvoiceLine,0,len(r.Items));for _,x:=range r.Items{items=append(items,service.InvoiceLine{FeeItemID:x.FeeItemID,Name:x.Name,Amount:x.Amount})};v,lines,e:=cn.service.CreateInvoice(service.CreateInvoiceInput{EnrollmentID:r.EnrollmentID,TermID:r.TermID,DueDate:due,Items:items});if e!=nil{financeError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(cn.service.DB(),c,&actor,"invoice.create","invoice",&v.ID,nil);c.JSON(http.StatusCreated,gin.H{"invoice":v,"items":lines})}
func(cn *FinanceController)ListInvoices(c *gin.Context){v,e:=cn.service.ListInvoices();if e!=nil{financeError(c,e);return};c.JSON(200,v)}
func(cn *FinanceController)GetInvoice(c *gin.Context){id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_invoice_id","invalid invoice id");return};v,items,e:=cn.service.GetInvoice(id);if e!=nil{financeError(c,e);return};c.JSON(200,gin.H{"invoice":v,"items":items})}

type paymentRequest struct{Amount float64 `json:"amount" binding:"required"`;Provider string `json:"provider"`;Reference string `json:"reference" binding:"required,max=120"`;Status string `json:"status"`}
func(cn *FinanceController)RecordPayment(c *gin.Context){id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_invoice_id","invalid invoice id");return};var r paymentRequest;if e=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};v,e:=cn.service.RecordPayment(id,r.Amount,r.Provider,r.Reference,r.Status);if e!=nil{financeError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(cn.service.DB(),c,&actor,"payment.record","payment",&v.ID,nil);c.JSON(http.StatusCreated,v)}
func(cn *FinanceController)ListPayments(c *gin.Context){v,e:=cn.service.ListPayments();if e!=nil{financeError(c,e);return};c.JSON(200,v)}
