package models

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

const (
    InvoiceStatusOpen = "open"
    InvoiceStatusPartiallyPaid = "partially_paid"
    InvoiceStatusPaid = "paid"
    InvoiceStatusCancelled = "cancelled"
    PaymentStatusPending = "pending"
    PaymentStatusSuccessful = "successful"
    PaymentStatusFailed = "failed"
)

type FeeItem struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    TermID uuid.UUID `gorm:"type:uuid;not null;index" json:"term_id"`
    Name string `gorm:"size:150;not null" json:"name"`
    Amount float64 `gorm:"not null" json:"amount"`
    Active bool `gorm:"not null;default:true;index" json:"active"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
func (f *FeeItem) BeforeCreate(tx *gorm.DB) error { f.ID = uuid.New(); return nil }

type Invoice struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    EnrollmentID uuid.UUID `gorm:"type:uuid;not null;index" json:"enrollment_id"`
    TermID uuid.UUID `gorm:"type:uuid;not null;index" json:"term_id"`
    InvoiceNumber string `gorm:"size:50;not null;uniqueIndex" json:"invoice_number"`
    TotalAmount float64 `gorm:"not null" json:"total_amount"`
    PaidAmount float64 `gorm:"not null;default:0" json:"paid_amount"`
    Balance float64 `gorm:"not null;default:0" json:"balance"`
    Status string `gorm:"size:20;not null;default:open;index" json:"status"`
    DueDate *time.Time `json:"due_date,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
func (i *Invoice) BeforeCreate(tx *gorm.DB) error { i.ID = uuid.New(); return nil }

type InvoiceItem struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    InvoiceID uuid.UUID `gorm:"type:uuid;not null;index" json:"invoice_id"`
    FeeItemID *uuid.UUID `gorm:"type:uuid;index" json:"fee_item_id,omitempty"`
    Name string `gorm:"size:150;not null" json:"name"`
    Amount float64 `gorm:"not null" json:"amount"`
    CreatedAt time.Time `json:"created_at"`
}
func (i *InvoiceItem) BeforeCreate(tx *gorm.DB) error { i.ID = uuid.New(); return nil }

type Payment struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    InvoiceID uuid.UUID `gorm:"type:uuid;not null;index" json:"invoice_id"`
    Amount float64 `gorm:"not null" json:"amount"`
    Provider string `gorm:"size:30;not null;default:manual;index" json:"provider"`
    Reference string `gorm:"size:120;not null;uniqueIndex" json:"reference"`
    Status string `gorm:"size:20;not null;default:pending;index" json:"status"`
    PaidAt *time.Time `json:"paid_at,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
func (p *Payment) BeforeCreate(tx *gorm.DB) error { p.ID = uuid.New(); return nil }