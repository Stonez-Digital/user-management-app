package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrSubscriptionPaymentUnavailable = errors.New("subscription payment unavailable")
	ErrSubscriptionPaymentPending = errors.New("a subscription checkout is already pending")
	ErrSubscriptionPaymentRecoveryRequired = errors.New("pending checkout has no saved authorization URL; verify its reference or contact support")
	ErrSubscriptionPaymentNotFound = errors.New("subscription payment not found")
	ErrSubscriptionPaymentInvalid = errors.New("subscription payment verification is invalid")
	ErrSubscriptionPaymentAlreadyActive = errors.New("school already has an active subscription")
)

type SubscriptionPaymentProvider interface {
	Name() string
	Initialize(context.Context, PaystackInitializeInput) (PaystackInitializeResult, error)
	Verify(string) (PaymentVerification, error)
}

type SubscriptionBillingService struct {
	db *gorm.DB
	provider SubscriptionPaymentProvider
	now func() time.Time
	tenantSchoolID uuid.UUID
	tenantUserID uuid.UUID
}

// WithTenantContext returns a request-scoped billing service. Each database
// phase opens a short transaction with transaction-local RLS claims; provider
// calls remain outside those transactions.
func (s *SubscriptionBillingService) WithTenantContext(schoolID, userID uuid.UUID) *SubscriptionBillingService {
	clone := *s
	clone.tenantSchoolID, clone.tenantUserID = schoolID, userID
	return &clone
}

func (s *SubscriptionBillingService) transaction(fn func(*gorm.DB) error) error {
	return tenantTransaction(s.db, s.tenantSchoolID, s.tenantUserID, fn)
}

// BillingContactEmail reads the authenticated school administrator's email
// under the same short tenant-scoped transaction used by billing operations.
func (s *SubscriptionBillingService) BillingContactEmail(schoolID, userID uuid.UUID) (string, error) {
	if schoolID == uuid.Nil || userID == uuid.Nil || (s.tenantSchoolID != uuid.Nil && s.tenantSchoolID != schoolID) {
		return "", ErrSubscriptionPaymentUnavailable
	}
	var email string
	err := tenantTransaction(s.db, schoolID, userID, func(tx *gorm.DB) error {
		var actor models.User
		if err := tx.Select("id", "email", "school_id", "active").
			Where("id = ? AND school_id = ?", userID, schoolID).First(&actor).Error; err != nil {
			return err
		}
		if !actor.Active || strings.TrimSpace(actor.Email) == "" {
			return ErrSubscriptionPaymentUnavailable
		}
		email = strings.TrimSpace(actor.Email)
		return nil
	})
	return email, err
}

type SubscriptionCheckout struct {
	Payment models.SubscriptionPayment
	Subscription models.SchoolSubscription
}

func NewSubscriptionBillingService(db *gorm.DB, provider SubscriptionPaymentProvider) *SubscriptionBillingService {
	return &SubscriptionBillingService{db: db, provider: provider, now: func() time.Time { return time.Now().UTC() }}
}

func (s *SubscriptionBillingService) CreateCheckout(schoolID, termID uuid.UUID, planCode, email, callbackURL string) (SubscriptionCheckout, error) {
	var result SubscriptionCheckout
	reusedPending := false
	planCode = strings.ToLower(strings.TrimSpace(planCode))
	email = strings.TrimSpace(email)
	if schoolID == uuid.Nil || termID == uuid.Nil || planCode == "" || email == "" || s.provider == nil {
		return result, ErrSubscriptionPaymentUnavailable
	}
	now := s.now().UTC()
	reference := "SUB-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))
	err := s.transaction(func(tx *gorm.DB) error {
		var school models.School
		if err := tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&school, "id = ?", schoolID).Error; err != nil { return err }
		if school.Status != models.SchoolStatusActive { return ErrSubscriptionPaymentUnavailable }

		var plan models.BillingPlan
		if err := tx.Where("code = ? AND active = ?", planCode, true).First(&plan).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) { return ErrSubscriptionPlanUnavailable }
			return err
		}
		if plan.Code == models.BillingPlanTrial || plan.Code == models.BillingPlanEnterprise || plan.AmountMinor <= 0 || plan.Currency != "NGN" {
			return ErrSubscriptionPlanUnavailable
		}

		var term models.Term
		if err := tx.Where("id = ? AND school_id = ?", termID, schoolID).First(&term).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) { return ErrSubscriptionTermUnavailable }
			return err
		}
		if term.Status != models.AcademicStatusActive || term.StartDate.After(now) || !term.EndDate.After(now) {
			return ErrSubscriptionTermUnavailable
		}
		var session models.AcademicSession
		if err := tx.Where("id = ? AND school_id = ?", term.AcademicSessionID, schoolID).First(&session).Error; err != nil || session.Status != models.AcademicStatusActive {
			return ErrSubscriptionTermUnavailable
		}

		var active int64
		if err := tx.Model(&models.SchoolSubscription{}).Where("school_id = ? AND status = ? AND expires_at > ?", schoolID, models.SubscriptionStatusActive, now).Count(&active).Error; err != nil { return err }
		if active > 0 { return ErrSubscriptionPaymentAlreadyActive }
		// A pending checkout is resumable when its authorization URL was saved.
		// Never create a second reference while an earlier payment may still settle.
		var pendingSub models.SchoolSubscription
		pendingErr := tx.Clauses(clause.Locking{Strength:"UPDATE"}).
			Where("school_id = ? AND status = ?", schoolID, models.SubscriptionStatusPending).
			First(&pendingSub).Error
		if pendingErr == nil {
			if pendingSub.PlanID != plan.ID || pendingSub.TermID != term.ID {
				return ErrSubscriptionPaymentPending
			}
			var pendingPayment models.SubscriptionPayment
			if err := tx.Where("school_id = ? AND subscription_id = ? AND status = ?", schoolID, pendingSub.ID, models.SubscriptionPaymentPending).First(&pendingPayment).Error; err != nil {
				return err
			}
			if pendingPayment.Provider != s.provider.Name() || pendingPayment.AuthorizationURL == "" {
				return ErrSubscriptionPaymentRecoveryRequired
			}
			result.Subscription, result.Payment = pendingSub, pendingPayment
			reusedPending = true
			return nil
		}
		if !errors.Is(pendingErr, gorm.ErrRecordNotFound) { return pendingErr }

		sub := models.SchoolSubscription{
			SchoolID:schoolID, PlanID:plan.ID, TermID:term.ID, Status:models.SubscriptionStatusPending,
			AmountMinor:plan.AmountMinor, Currency:"NGN", StartsAt:now, ExpiresAt:term.EndDate.UTC(),
		}
		if err := tx.Create(&sub).Error; err != nil { return err }
		payment := models.SubscriptionPayment{
			SchoolID:schoolID, SubscriptionID:sub.ID, Provider:s.provider.Name(), Reference:reference,
			AmountMinor:plan.AmountMinor, Currency:"NGN", Status:models.SubscriptionPaymentPending,
		}
		if err := tx.Create(&payment).Error; err != nil { return err }
		result.Subscription, result.Payment = sub, payment
		return nil
	})
	if err != nil { return SubscriptionCheckout{}, err }
	if reusedPending { return result, nil }

	metadata := map[string]interface{}{"school_id":schoolID.String(),"subscription_id":result.Subscription.ID.String(),"plan_code":planCode}
	checkout, err := s.provider.Initialize(context.Background(), PaystackInitializeInput{
		Email:email, AmountMinor:result.Payment.AmountMinor, Reference:reference, CallbackURL:callbackURL, Metadata:metadata,
	})
	if err != nil {
		// Initialization failures can be ambiguous (for example, a network
		// timeout after Paystack created the transaction). Keep the local
		// payment pending so a late successful payment can still be verified.
		return result, ErrSubscriptionPaymentRecoveryRequired
	}
	if checkout.Reference != reference || checkout.AuthorizationURL == "" || checkout.AccessCode == "" {
		return SubscriptionCheckout{}, ErrSubscriptionPaymentUnavailable
	}
	if err := s.transaction(func(tx *gorm.DB) error {
		updated := tx.Model(&models.SubscriptionPayment{}).
			Where("id = ? AND school_id = ? AND reference = ? AND status = ?", result.Payment.ID, schoolID, reference, models.SubscriptionPaymentPending).
			Updates(map[string]interface{}{"authorization_url": checkout.AuthorizationURL, "access_code": checkout.AccessCode})
		if updated.Error != nil { return updated.Error }
		if updated.RowsAffected != 1 { return ErrSubscriptionPaymentRecoveryRequired }
		return nil
	}); err != nil {
		return result, err
	}
	result.Payment.AuthorizationURL, result.Payment.AccessCode = checkout.AuthorizationURL, checkout.AccessCode
	return result, nil
}

func (s *SubscriptionBillingService) VerifyCheckout(schoolID uuid.UUID, reference string) (SubscriptionCheckout, error) {
	var result SubscriptionCheckout
	reference = strings.TrimSpace(reference)
	if schoolID == uuid.Nil || reference == "" || s.provider == nil { return result, ErrSubscriptionPaymentInvalid }
	var payment models.SubscriptionPayment
	err := s.transaction(func(tx *gorm.DB) error {
		if err := tx.Where("school_id = ? AND reference = ?", schoolID, reference).First(&payment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) { return ErrSubscriptionPaymentNotFound }
			return err
		}
		if payment.Provider != s.provider.Name() { return ErrSubscriptionPaymentInvalid }
		if payment.Status == models.SubscriptionPaymentSucceeded {
			if err := tx.Preload("Plan").Where("id = ? AND school_id = ?", payment.SubscriptionID, schoolID).First(&result.Subscription).Error; err != nil { return err }
			result.Payment = payment
			return nil
		}
		if payment.Status != models.SubscriptionPaymentPending { return ErrSubscriptionPaymentInvalid }
		return nil
	})
	if err != nil { return result, err }
	if payment.Status == models.SubscriptionPaymentSucceeded { return result, nil }

	verified, err := s.provider.Verify(reference)
	if err != nil || verified.Provider != s.provider.Name() || verified.Reference != reference || verified.AmountMinor != payment.AmountMinor ||
		!strings.EqualFold(verified.Currency,"NGN") || verified.Status != "succeeded" || verified.PaidAt.IsZero() {
		return result, ErrSubscriptionPaymentInvalid
	}
	metadata, err := json.Marshal(verified.Metadata)
	if err != nil { return result, ErrSubscriptionPaymentInvalid }
	now := s.now().UTC()
	err = s.transaction(func(tx *gorm.DB) error {
		var locked models.SubscriptionPayment
		if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).Where("id = ? AND school_id = ?",payment.ID,schoolID).First(&locked).Error;err!=nil{return err}
		if locked.Status == models.SubscriptionPaymentSucceeded {
			if err:=tx.Preload("Plan").Where("id = ? AND school_id = ?",locked.SubscriptionID,schoolID).First(&result.Subscription).Error;err!=nil{return err}
			result.Payment=locked
			return nil
		}
		if locked.Status != models.SubscriptionPaymentPending || locked.AmountMinor != verified.AmountMinor || locked.Reference != verified.Reference {
			return ErrSubscriptionPaymentInvalid
		}
		var sub models.SchoolSubscription
		if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).Where("id = ? AND school_id = ?",locked.SubscriptionID,schoolID).First(&sub).Error;err!=nil{return err}
		var active int64
		if err:=tx.Model(&models.SchoolSubscription{}).Where("school_id = ? AND status = ? AND expires_at > ? AND id <> ?",schoolID,models.SubscriptionStatusActive,now,sub.ID).Count(&active).Error;err!=nil{return err}
		if active>0{return ErrSubscriptionPaymentAlreadyActive}
		if sub.Status != models.SubscriptionStatusPending || sub.AmountMinor != verified.AmountMinor || sub.Currency != "NGN" {
			return ErrSubscriptionPaymentInvalid
		}
		if err:=tx.Model(&sub).Updates(map[string]interface{}{"status":models.SubscriptionStatusActive,"starts_at":now}).Error;err!=nil{return err}
		paidAt:=verified.PaidAt.UTC()
		if err:=tx.Model(&locked).Updates(map[string]interface{}{"status":models.SubscriptionPaymentSucceeded,"paid_at":paidAt,"metadata":string(metadata)}).Error;err!=nil{return err}
		event:=models.SubscriptionEvent{SchoolID:schoolID,SubscriptionID:sub.ID,EventType:"subscription_activated",FromStatus:models.SubscriptionStatusPending,ToStatus:models.SubscriptionStatusActive,Metadata:fmt.Sprintf("{\"payment_reference\":%q}",reference)}
		if err:=tx.Create(&event).Error;err!=nil{return err}
		if err:=tx.Preload("Plan").First(&result.Subscription,"id = ?",sub.ID).Error;err!=nil{return err}
		locked.Status=models.SubscriptionPaymentSucceeded;locked.PaidAt=&paidAt;locked.Metadata=string(metadata)
		result.Payment=locked
		return nil
	})
	if err != nil { return SubscriptionCheckout{}, err }
	return result,nil
}
