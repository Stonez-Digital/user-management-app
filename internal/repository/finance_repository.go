package repository

import (
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type FeeItemRepository interface { Create(models.FeeItem)(models.FeeItem,error); List(*uuid.UUID)([]models.FeeItem,error); Get(uuid.UUID)(models.FeeItem,error); Update(models.FeeItem) error; Delete(uuid.UUID) error }
type InvoiceRepository interface { Create(models.Invoice)(models.Invoice,error); List()([]models.Invoice,error); Get(uuid.UUID)(models.Invoice,error); Update(models.Invoice) error }
type PaymentRepository interface { Create(models.Payment)(models.Payment,error); ListByInvoice(uuid.UUID)([]models.Payment,error); GetByReference(string)(models.Payment,error) }

type feeItemRepo struct{db *gorm.DB}
func NewFeeItemRepository(db *gorm.DB) FeeItemRepository{return &feeItemRepo{db}}
func(r *feeItemRepo)Create(v models.FeeItem)(models.FeeItem,error){return v,r.db.Create(&v).Error}
func(r *feeItemRepo)List(termID *uuid.UUID)([]models.FeeItem,error){var v []models.FeeItem;q:=r.db.Preload("Term").Order("name ASC");if termID!=nil{q=q.Where("term_id = ?",*termID)};return v,q.Find(&v).Error}
func(r *feeItemRepo)Get(id uuid.UUID)(models.FeeItem,error){var v models.FeeItem;return v,r.db.Preload("Term").First(&v,"id = ?",id).Error}
func(r *feeItemRepo)Update(v models.FeeItem)error{return r.db.Save(&v).Error}
func(r *feeItemRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.FeeItem{},"id = ?",id).Error}

type invoiceRepo struct{db *gorm.DB}
func NewInvoiceRepository(db *gorm.DB) InvoiceRepository{return &invoiceRepo{db}}
func(r *invoiceRepo)Create(v models.Invoice)(models.Invoice,error){return v,r.db.Create(&v).Error}
func(r *invoiceRepo)List()([]models.Invoice,error){var v []models.Invoice;return v,r.db.Preload("StudentEnrollment.Student").Preload("Term").Preload("Lines").Preload("Lines.FeeItem").Preload("Payments").Order("created_at DESC").Find(&v).Error}
func(r *invoiceRepo)Get(id uuid.UUID)(models.Invoice,error){var v models.Invoice;e:=r.db.Preload("StudentEnrollment.Student").Preload("Term").Preload("Lines").Preload("Lines.FeeItem").Preload("Payments").First(&v,"id = ?",id).Error;return v,e}
func(r *invoiceRepo)Update(v models.Invoice)error{return r.db.Save(&v).Error}

type paymentRepo struct{db *gorm.DB}
func NewPaymentRepository(db *gorm.DB) PaymentRepository{return &paymentRepo{db}}
func(r *paymentRepo)Create(v models.Payment)(models.Payment,error){return v,r.db.Create(&v).Error}
func(r *paymentRepo)ListByInvoice(id uuid.UUID)([]models.Payment,error){var v []models.Payment;return v,r.db.Where("invoice_id = ?",id).Order("created_at DESC").Find(&v).Error}
func(r *paymentRepo)GetByReference(ref string)(models.Payment,error){var v models.Payment;return v,r.db.Where("reference = ?",ref).First(&v).Error}
