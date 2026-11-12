package models

import (
	"time"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	BillingPlanTrial = "trial"
	BillingPlanBasic = "basic"
	BillingPlanProfessional = "professional"
	BillingPlanPremium = "premium"
	BillingPlanEnterprise = "enterprise"
	SubscriptionStatusActive = "active"
	SubscriptionStatusExpired = "expired"
	SubscriptionStatusCancelled = "cancelled"
	SubscriptionStatusPending = "pending_payment"
	SubscriptionEventTrialStarted = "trial_started"
	SubscriptionEventExpired = "subscription_expired"
)

type BillingPlan struct {
	ID uuid.UUID "gorm:\"type:uuid;primaryKey\" json:\"id\""
	Code string "gorm:\"size:30;not null;uniqueIndex\" json:\"code\""
	Name string "gorm:\"size:80;not null\" json:\"name\""
	AmountMinor int64 "gorm:\"not null;default:0\" json:\"amount_minor\""
	Currency string "gorm:\"size:3;not null;default:NGN\" json:\"currency\""
	BillingInterval string "gorm:\"size:20;not null;default:term\" json:\"billing_interval\""
	MaxStudents *int "json:\"max_students,omitempty\""
	MaxTeachers *int "json:\"max_teachers,omitempty\""
	StorageLimitBytes *int64 "json:\"storage_limit_bytes,omitempty\""
	Active bool "gorm:\"not null;default:true;index\" json:\"active\""
	SortOrder int "gorm:\"not null;default:0\" json:\"sort_order\""
	CreatedAt time.Time "json:\"created_at\""
	UpdatedAt time.Time "json:\"updated_at\""
}

func (p *BillingPlan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil { p.ID = uuid.New() }
	if p.Currency == "" { p.Currency = "NGN" }
	if p.BillingInterval == "" { p.BillingInterval = "term" }
	return nil
}

type SchoolSubscription struct {
	ID uuid.UUID "gorm:\"type:uuid;primaryKey\" json:\"id\""
	SchoolID uuid.UUID "gorm:\"type:uuid;not null;index\" json:\"school_id\""
	School School "gorm:\"foreignKey:SchoolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT\" json:\"-\""
	PlanID uuid.UUID "gorm:\"type:uuid;not null;index\" json:\"plan_id\""
	Plan BillingPlan "gorm:\"foreignKey:PlanID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT\" json:\"plan,omitempty\""
	TermID uuid.UUID "gorm:\"type:uuid;not null;index\" json:\"term_id\""
	Term Term "gorm:\"foreignKey:TermID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT\" json:\"-\""
	Status string "gorm:\"size:24;not null;index\" json:\"status\""
	IsTrial bool "gorm:\"not null;default:false;index\" json:\"is_trial\""
	AmountMinor int64 "gorm:\"not null;default:0\" json:\"amount_minor\""
	Currency string "gorm:\"size:3;not null;default:NGN\" json:\"currency\""
	StartsAt time.Time "gorm:\"not null;index\" json:\"starts_at\""
	ExpiresAt time.Time "gorm:\"not null;index\" json:\"expires_at\""
	CancelledAt *time.Time "json:\"cancelled_at,omitempty\""
	CreatedAt time.Time "json:\"created_at\""
	UpdatedAt time.Time "json:\"updated_at\""
}

func (s *SchoolSubscription) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil { s.ID = uuid.New() }
	if s.Currency == "" { s.Currency = "NGN" }
	return nil
}

type SubscriptionEvent struct {
	ID uuid.UUID "gorm:\"type:uuid;primaryKey\" json:\"id\""
	SchoolID uuid.UUID "gorm:\"type:uuid;not null;index\" json:\"school_id\""
	SubscriptionID uuid.UUID "gorm:\"type:uuid;not null;index\" json:\"subscription_id\""
	ActorID *uuid.UUID "gorm:\"type:uuid\" json:\"actor_id,omitempty\""
	EventType string "gorm:\"size:50;not null;index\" json:\"event_type\""
	FromStatus string "gorm:\"size:24\" json:\"from_status,omitempty\""
	ToStatus string "gorm:\"size:24\" json:\"to_status,omitempty\""
	Metadata string "gorm:\"type:text\" json:\"metadata,omitempty\""
	CreatedAt time.Time "json:\"created_at\""
}

func (e *SubscriptionEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil { e.ID = uuid.New() }
	return nil
}
