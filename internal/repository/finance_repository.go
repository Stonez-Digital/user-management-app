package repository

import (
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type FeeItemRepository interface {
    Create(uuid.UUID, models.FeeItem) (models.FeeItem, error)
    List(uuid.UUID, *uuid.UUID) ([]models.FeeItem, error)
    Get(uuid.UUID, uuid.UUID) (models.FeeItem, error)
    Update(uuid.UUID, models.FeeItem) error
    Delete(uuid.UUID, uuid.UUID) error
}
type InvoiceRepository interface {
    Create(uuid.UUID, models.Invoice) (models.Invoice, error)
    List(uuid.UUID) ([]models.Invoice, error)
    Get(uuid.UUID, uuid.UUID) (models.Invoice, error)
    Update(uuid.UUID, models.Invoice) error
}
type PaymentRepository interface {
    Create(uuid.UUID, models.Payment) (models.Payment, error)
    ListByInvoice(uuid.UUID, uuid.UUID) ([]models.Payment, error)
    GetByReference(uuid.UUID, string) (models.Payment, error)
}

type feeItemRepo struct{db *gorm.DB}
func NewFeeItemRepository(db *gorm.DB) FeeItemRepository{return &feeItemRepo{db}}
func(r *feeItemRepo)Create(schoolID uuid.UUID,v models.FeeItem)(models.FeeItem,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *feeItemRepo)List(schoolID uuid.UUID,termID *uuid.UUID)([]models.FeeItem,error){var v []models.FeeItem;q:=r.db.Preload("Term").Where("school_id = ?",schoolID).Order("name ASC");if termID!=nil{q=q.Where("term_id = ?",*termID)};return v,q.Find(&v).Error}
func(r *feeItemRepo)Get(schoolID,id uuid.UUID)(models.FeeItem,error){var v models.FeeItem;return v,r.db.Preload("Term").First(&v,"id = ? AND school_id = ?",id,schoolID).Error}
func(r *feeItemRepo)Update(schoolID uuid.UUID,v models.FeeItem)error{v.SchoolID=schoolID;return r.db.Model(&models.FeeItem{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(map[string]interface{}{"term_id":v.TermID,"name":v.Name,"description":v.Description,"amount":v.Amount,"active":v.Active}).Error}
func(r *feeItemRepo)Delete(schoolID,id uuid.UUID)error{return r.db.Delete(&models.FeeItem{},"id = ? AND school_id = ?",id,schoolID).Error}

type invoiceRepo struct{db *gorm.DB}
func NewInvoiceRepository(db *gorm.DB) InvoiceRepository{return &invoiceRepo{db}}
func(r *invoiceRepo)Create(schoolID uuid.UUID,v models.Invoice)(models.Invoice,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *invoiceRepo)List(schoolID uuid.UUID)([]models.Invoice,error){var v []models.Invoice;return v,r.db.Where("school_id = ?",schoolID).Preload("StudentEnrollment.Student").Preload("Term").Preload("Lines", "school_id = ?", schoolID).Preload("Lines.FeeItem", "school_id = ?", schoolID).Preload("Payments", "school_id = ?", schoolID).Order("created_at DESC").Find(&v).Error}
func(r *invoiceRepo)Get(schoolID,id uuid.UUID)(models.Invoice,error){var v models.Invoice;e:=r.db.Where("id = ? AND school_id = ?",id,schoolID).Preload("StudentEnrollment.Student").Preload("Term").Preload("Lines", "school_id = ?", schoolID).Preload("Lines.FeeItem", "school_id = ?", schoolID).Preload("Payments", "school_id = ?", schoolID).First(&v).Error;return v,e}
func(r *invoiceRepo)Update(schoolID uuid.UUID,v models.Invoice)error{v.SchoolID=schoolID;return r.db.Model(&models.Invoice{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(map[string]interface{}{"invoice_number":v.InvoiceNumber,"due_date":v.DueDate,"total_amount":v.TotalAmount,"paid_amount":v.PaidAmount,"balance":v.Balance,"status":v.Status}).Error}

type paymentRepo struct{db *gorm.DB}
func NewPaymentRepository(db *gorm.DB) PaymentRepository{return &paymentRepo{db}}
func(r *paymentRepo)Create(schoolID uuid.UUID,v models.Payment)(models.Payment,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *paymentRepo)ListByInvoice(schoolID,invoiceID uuid.UUID)([]models.Payment,error){var v []models.Payment;return v,r.db.Where("school_id = ? AND invoice_id = ?",schoolID,invoiceID).Order("created_at DESC").Find(&v).Error}
func(r *paymentRepo)GetByReference(schoolID uuid.UUID,ref string)(models.Payment,error){var v models.Payment;return v,r.db.Where("school_id = ? AND reference = ?",schoolID,ref).First(&v).Error}
