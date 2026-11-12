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
	err := s.db.Transaction(func(tx *gorm.DB) error {
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
		return SubscriptionCheckout{}, ErrSubscriptionPaymentRecoveryRequired
	}
	if checkout.Reference != reference || checkout.AuthorizationURL == "" || checkout.AccessCode == "" {
		return SubscriptionCheckout{}, ErrSubscriptionPaymentUnavailable
	}
	if err := s.db.Model(&models.SubscriptionPayment{}).Where("id = ? AND status = ?",result.Payment.ID,models.SubscriptionPaymentPending).
		Updates(map[string]interface{}{"authorization_url":checkout.AuthorizationURL,"access_code":checkout.AccessCode}).Error; err != nil {
		return SubscriptionCheckout{}, err
	}
	result.Payment.AuthorizationURL, result.Payment.AccessCode = checkout.AuthorizationURL, checkout.AccessCode
	return result, nil
}

func (s *SubscriptionBillingService) VerifyCheckout(schoolID uuid.UUID, reference string) (SubscriptionCheckout, error) {
	var result SubscriptionCheckout
	reference = strings.TrimSpace(reference)
	if schoolID == uuid.Nil || reference == "" || s.provider == nil { return result, ErrSubscriptionPaymentInvalid }
	var payment models.SubscriptionPayment
	if err := s.db.Where("school_id = ? AND reference = ?",schoolID,reference).First(&payment).Error; err != nil {
		if errors.Is(err,gorm.ErrRecordNotFound) { return result, ErrSubscriptionPaymentNotFound }
		return result, err
	}
	if payment.Provider != s.provider.Name() { return result, ErrSubscriptionPaymentInvalid }
	if payment.Status == models.SubscriptionPaymentSucceeded {
		if err:=s.db.Preload("Plan").Where("id = ? AND school_id = ?",payment.SubscriptionID,schoolID).First(&result.Subscription).Error;err!=nil{return result,err}
		result.Payment=payment
		return result,nil
	}
	if payment.Status != models.SubscriptionPaymentPending { return result, ErrSubscriptionPaymentInvalid }

	verified, err := s.provider.Verify(reference)
	if err != nil || verified.Provider != s.provider.Name() || verified.Reference != reference || verified.AmountMinor != payment.AmountMinor ||
		!strings.EqualFold(verified.Currency,"NGN") || verified.Status != "succeeded" || verified.PaidAt.IsZero() {
		return result, ErrSubscriptionPaymentInvalid
	}
	metadata, err := json.Marshal(verified.Metadata)
	if err != nil { return result, ErrSubscriptionPaymentInvalid }
	now := s.now().UTC()
	err = s.db.Transaction(func(tx *gorm.DB) error {
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
