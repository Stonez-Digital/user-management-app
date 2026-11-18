package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrSubscriptionInvalid = errors.New("invalid subscription request")
	ErrSubscriptionTermUnavailable = errors.New("a current academic term is required to start the trial")
	ErrSubscriptionTrialUsed = errors.New("the school has already used its free trial")
	ErrSubscriptionAlreadyActive = errors.New("the school already has an active subscription")
	ErrSubscriptionNotFound = errors.New("active subscription not found")
	ErrSubscriptionPlanUnavailable = errors.New("billing plan is unavailable")
	ErrSubscriptionQuotaExceeded = errors.New("subscription quota exceeded")
)

type SubscriptionService struct {
	db *gorm.DB
	now func() time.Time
}

func NewSubscriptionService(db *gorm.DB) *SubscriptionService {
	return &SubscriptionService{db: db, now: func() time.Time { return time.Now().UTC() }}
}

func NewSubscriptionServiceWithClock(db *gorm.DB, clock func() time.Time) *SubscriptionService {
	if clock == nil { clock = func() time.Time { return time.Now().UTC() } }
	return &SubscriptionService{db: db, now: clock}
}

func (s *SubscriptionService) ListPlans() ([]models.BillingPlan, error) {
	var plans []models.BillingPlan
	err := s.db.Where("active = ?", true).Order("sort_order ASC, amount_minor ASC").Find(&plans).Error
	return plans, err
}

// StartTrial requires a current academic term belonging to the same school.
// requested_plan on the school record remains a preference, not activation.
func (s *SubscriptionService) StartTrial(schoolID, termID uuid.UUID) (models.SchoolSubscription, error) {
	var subscription models.SchoolSubscription
	if schoolID == uuid.Nil || termID == uuid.Nil { return subscription, ErrSubscriptionInvalid }
	now := s.now().UTC()

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var school models.School
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&school, "id = ?", schoolID).Error; err != nil { return err }
		if school.Status != models.SchoolStatusActive { return ErrSubscriptionInvalid }

		// Clear a stale active row before inserting the new trial. The school row
		// lock serializes this transition against concurrent trial starts.
		var stale []models.SchoolSubscription
		if err := tx.Where("school_id = ? AND status = ? AND expires_at <= ?", schoolID, models.SubscriptionStatusActive, now).Find(&stale).Error; err != nil { return err }
		for _, previous := range stale {
			if err := tx.Model(&models.SchoolSubscription{}).Where("id = ? AND status = ?", previous.ID, models.SubscriptionStatusActive).Update("status", models.SubscriptionStatusExpired).Error; err != nil { return err }
			event := models.SubscriptionEvent{SchoolID: schoolID, SubscriptionID: previous.ID, EventType: models.SubscriptionEventExpired, FromStatus: models.SubscriptionStatusActive, ToStatus: models.SubscriptionStatusExpired}
			if err := tx.Create(&event).Error; err != nil { return err }
		}

		var term models.Term
		if err := tx.Where("id = ? AND school_id = ?", termID, schoolID).First(&term).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) { return ErrSubscriptionTermUnavailable }
			return err
		}
		if term.StartDate.After(now) || !term.EndDate.After(now) || term.Status != models.AcademicStatusActive {
			return ErrSubscriptionTermUnavailable
		}
		var session models.AcademicSession
		if err := tx.Where("id = ? AND school_id = ?", term.AcademicSessionID, schoolID).First(&session).Error; err != nil {
			return ErrSubscriptionTermUnavailable
		}
		if session.Status != models.AcademicStatusActive {
			return ErrSubscriptionTermUnavailable
		}

		var trialCount int64
		if err := tx.Model(&models.SchoolSubscription{}).Where("school_id = ? AND is_trial = ?", schoolID, true).Count(&trialCount).Error; err != nil { return err }
		if trialCount > 0 { return ErrSubscriptionTrialUsed }

		var activeCount int64
		if err := tx.Model(&models.SchoolSubscription{}).Where("school_id = ? AND status = ? AND expires_at > ?", schoolID, models.SubscriptionStatusActive, now).Count(&activeCount).Error; err != nil { return err }
		if activeCount > 0 { return ErrSubscriptionAlreadyActive }

		var plan models.BillingPlan
		if err := tx.Where("code = ? AND active = ?", models.BillingPlanTrial, true).First(&plan).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) { return ErrSubscriptionPlanUnavailable }
			return err
		}

		subscription = models.SchoolSubscription{
			SchoolID: schoolID, PlanID: plan.ID, TermID: term.ID,
			Status: models.SubscriptionStatusActive, IsTrial: true,
			AmountMinor: 0, Currency: plan.Currency, StartsAt: now,
			ExpiresAt: term.EndDate.UTC(),
		}
		if err := tx.Create(&subscription).Error; err != nil { return err }
		event := models.SubscriptionEvent{
			SchoolID: schoolID, SubscriptionID: subscription.ID,
			EventType: models.SubscriptionEventTrialStarted,
			ToStatus: models.SubscriptionStatusActive,
		}
		return tx.Create(&event).Error
	})
	if err != nil { return models.SchoolSubscription{}, err }
	return subscription, nil
}

// CurrentSubscription expires an ended subscription and records the transition once.
func (s *SubscriptionService) CurrentSubscription(schoolID uuid.UUID) (models.SchoolSubscription, error) {
	if schoolID == uuid.Nil { return models.SchoolSubscription{}, ErrSubscriptionInvalid }
	now := s.now().UTC()
	var result models.SchoolSubscription
	expired := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var current models.SchoolSubscription
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Plan").
			Where("school_id = ? AND status = ?", schoolID, models.SubscriptionStatusActive).
			Order("created_at DESC").First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) { return ErrSubscriptionNotFound }
		if err != nil { return err }
		if !current.ExpiresAt.After(now) {
			updated := tx.Model(&models.SchoolSubscription{}).
				Where("id = ? AND status = ?", current.ID, models.SubscriptionStatusActive).
				Update("status", models.SubscriptionStatusExpired)
			if updated.Error != nil { return updated.Error }
			if updated.RowsAffected == 1 {
				event := models.SubscriptionEvent{
					SchoolID: schoolID, SubscriptionID: current.ID,
					EventType: models.SubscriptionEventExpired,
					FromStatus: models.SubscriptionStatusActive,
					ToStatus: models.SubscriptionStatusExpired,
				}
				if err := tx.Create(&event).Error; err != nil { return err }
			}
			expired = true
			return nil
		}
		result = current
		return nil
	})
	if err != nil { return models.SchoolSubscription{}, err }
	if expired { return models.SchoolSubscription{}, ErrSubscriptionNotFound }
	return result, nil
}

// CheckQuota provides a server-side entitlement check for API handlers.
// Usage is supplied by the caller from a school-scoped database count.
func (s *SubscriptionService) CheckQuota(schoolID uuid.UUID, metric string, currentUsage int64) error {
	if currentUsage < 0 { return ErrSubscriptionInvalid }
	subscription, err := s.CurrentSubscription(schoolID)
	if err != nil { return err }
	var limit *int64
	switch metric {
	case "students":
		if subscription.Plan.MaxStudents != nil { v := int64(*subscription.Plan.MaxStudents); limit = &v }
	case "teachers":
		if subscription.Plan.MaxTeachers != nil { v := int64(*subscription.Plan.MaxTeachers); limit = &v }
	case "storage_bytes":
		limit = subscription.Plan.StorageLimitBytes
	default:
		return fmt.Errorf("%w: unknown quota %q", ErrSubscriptionInvalid, metric)
	}
	if limit != nil && currentUsage >= *limit { return ErrSubscriptionQuotaExceeded }
	return nil
}
