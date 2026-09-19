package repository

import (
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type FeeItemRepository interface {
    Create(models.FeeItem) (models.FeeItem, error)
    List() ([]models.FeeItem, error)
    Get(uuid.UUID) (models.FeeItem, error)
    Update(models.FeeItem) error
    Delete(uuid.UUID) error
}
type feeItemRepo struct{ db *gorm.DB }
func NewFeeItemRepository(db *gorm.DB) FeeItemRepository { return &feeItemRepo{db} }
func (r *feeItemRepo) Create(v models.FeeItem)(models.FeeItem,error){ return v,r.db.Create(&v).Error }
func (r *feeItemRepo) List()([]models.FeeItem,error){var v []models.FeeItem; e:=r.db.Order("created_at DESC").Find(&v).Error; return v,e}
func (r *feeItemRepo) Get(id uuid.UUID)(models.FeeItem,error){var v models.FeeItem; e:=r.db.First(&v,"id=?",id).Error; return v,e}
func (r *feeItemRepo) Update(v models.FeeItem)error{return r.db.Save(&v).Error}
func (r *feeItemRepo) Delete(id uuid.UUID)error{return r.db.Delete(&models.FeeItem{},"id=?",id).Error}

type InvoiceRepository interface {
    Create(models.Invoice) (models.Invoice, error)
    List() ([]models.Invoice, error)
    Get(uuid.UUID) (models.Invoice, error)
    Update(models.Invoice) error
    AddItem(models.InvoiceItem) error
    Items(uuid.UUID) ([]models.InvoiceItem, error)
}
type invoiceRepo struct{ db *gorm.DB }
func NewInvoiceRepository(db *gorm.DB) InvoiceRepository { return &invoiceRepo{db} }
func (r *invoiceRepo) Create(v models.Invoice)(models.Invoice,error){return v,r.db.Create(&v).Error}
func (r *invoiceRepo) List()([]models.Invoice,error){var v []models.Invoice;e:=r.db.Order("created_at DESC").Find(&v).Error;return v,e}
func (r *invoiceRepo) Get(id uuid.UUID)(models.Invoice,error){var v models.Invoice;e:=r.db.First(&v,"id=?",id).Error;return v,e}
func (r *invoiceRepo) Update(v models.Invoice)error{return r.db.Save(&v).Error}
func (r *invoiceRepo) AddItem(v models.InvoiceItem)error{return r.db.Create(&v).Error}
func (r *invoiceRepo) Items(id uuid.UUID)([]models.InvoiceItem,error){var v []models.InvoiceItem;e:=r.db.Where("invoice_id=?",id).Order("created_at").Find(&v).Error;return v,e}

type PaymentRepository interface {
    Create(models.Payment) (models.Payment, error)
    List() ([]models.Payment, error)
    FindByReference(string) (models.Payment, error)
}
type paymentRepo struct{ db *gorm.DB }
func NewPaymentRepository(db *gorm.DB) PaymentRepository { return &paymentRepo{db} }
func (r *paymentRepo) Create(v models.Payment)(models.Payment,error){return v,r.db.Create(&v).Error}
func (r *paymentRepo) List()([]models.Payment,error){var v []models.Payment;e:=r.db.Order("created_at DESC").Find(&v).Error;return v,e}
func (r *paymentRepo) FindByReference(ref string)(models.Payment,error){var v models.Payment;e:=r.db.Where("reference=?",ref).First(&v).Error;return v,e}