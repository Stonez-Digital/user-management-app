package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	SubscriptionPaymentPending = "pending"
	SubscriptionPaymentSucceeded = "succeeded"
	SubscriptionPaymentFailed = "failed"
)

type SubscriptionPayment struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SchoolID uuid.UUID `gorm:"type:uuid;not null;index" json:"school_id"`
	SubscriptionID uuid.UUID `gorm:"type:uuid;not null;index" json:"subscription_id"`
	Provider string `gorm:"size:30;not null" json:"provider"`
	Reference string `gorm:"size:100;not null;uniqueIndex" json:"reference"`
	AmountMinor int64 `gorm:"not null" json:"amount_minor"`
	Currency string `gorm:"size:3;not null;default:NGN" json:"currency"`
	Status string `gorm:"size:24;not null;index" json:"status"`
	AuthorizationURL string `gorm:"size:1000" json:"authorization_url,omitempty"`
	AccessCode string `gorm:"size:255" json:"access_code,omitempty"`
	PaidAt *time.Time `json:"paid_at,omitempty"`
	Metadata string `gorm:"type:text" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (p *SubscriptionPayment) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil { p.ID = uuid.New() }
	if p.Currency == "" { p.Currency = "NGN" }
	if p.Status == "" { p.Status = SubscriptionPaymentPending }
	return nil
}
