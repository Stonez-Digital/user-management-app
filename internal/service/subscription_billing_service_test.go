package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type subscriptionBillingFakeProvider struct {
	initialized PaystackInitializeInput
	verification PaymentVerification
	initializeErr error
	verifyErr error
	initializeCalls int
}
func (p *subscriptionBillingFakeProvider) Name() string { return "paystack" }
func (p *subscriptionBillingFakeProvider) Initialize(_ context.Context, in PaystackInitializeInput) (PaystackInitializeResult,error) {
	p.initialized=in
	p.initializeCalls++
	if p.initializeErr!=nil{return PaystackInitializeResult{},p.initializeErr}
	return PaystackInitializeResult{AuthorizationURL:"https://checkout.paystack.com/example",AccessCode:"access-example",Reference:in.Reference},nil
}
func (p *subscriptionBillingFakeProvider) Verify(reference string)(PaymentVerification,error){
	if p.verifyErr!=nil{return PaymentVerification{},p.verifyErr}
	v:=p.verification;v.Reference=reference;return v,nil
}

func subscriptionBillingTestDB(t *testing.T)(*gorm.DB,uuid.UUID,uuid.UUID){
	t.Helper()
	db,err:=gorm.Open(sqlite.Open("file:subscription-billing-test-"+uuid.NewString()+"?mode=memory&cache=shared"),&gorm.Config{})
	if err!=nil{t.Fatal(err)}
	if err:=db.AutoMigrate(&models.School{},&models.AcademicSession{},&models.Term{},&models.BillingPlan{},&models.SchoolSubscription{},&models.SubscriptionEvent{},&models.SubscriptionPayment{});err!=nil{t.Fatal(err)}
	school:=models.School{Name:"Billing Test School",Code:"BILLING-TEST",Status:models.SchoolStatusActive}
	if err:=db.Create(&school).Error;err!=nil{t.Fatal(err)}
	session:=models.AcademicSession{SchoolID:school.ID,Name:"2026/2027",StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,7,31,0,0,0,0,time.UTC),Status:models.AcademicStatusActive}
	if err:=db.Create(&session).Error;err!=nil{t.Fatal(err)}
	term:=models.Term{SchoolID:school.ID,AcademicSessionID:session.ID,Name:"First",StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2026,12,20,0,0,0,0,time.UTC),Status:models.AcademicStatusActive}
	if err:=db.Create(&term).Error;err!=nil{t.Fatal(err)}
	plan:=models.BillingPlan{Code:models.BillingPlanBasic,Name:"Basic",AmountMinor:6500000,Currency:"NGN",BillingInterval:"term",Active:true}
	if err:=db.Create(&plan).Error;err!=nil{t.Fatal(err)}
	professional:=models.BillingPlan{Code:models.BillingPlanProfessional,Name:"Professional",AmountMinor:10000000,Currency:"NGN",BillingInterval:"term",Active:true}
	if err:=db.Create(&professional).Error;err!=nil{t.Fatal(err)}
	return db,school.ID,term.ID
}

func TestSubscriptionCheckoutPersistsAndActivatesOnlyAfterVerifiedPayment(t *testing.T){
	db,schoolID,termID:=subscriptionBillingTestDB(t)
	now:=time.Date(2026,11,12,10,0,0,0,time.UTC)
	provider:=&subscriptionBillingFakeProvider{verification:PaymentVerification{Provider:"paystack",AmountMinor:6500000,Currency:"NGN",Status:"succeeded",PaidAt:now}}
	svc:=NewSubscriptionBillingService(db,provider);svc.now=func()time.Time{return now}
	checkout,err:=svc.CreateCheckout(schoolID,termID,models.BillingPlanBasic,"admin@school.test","")
	if err!=nil{t.Fatal(err)}
	if checkout.Payment.Status!=models.SubscriptionPaymentPending||checkout.Subscription.Status!=models.SubscriptionStatusPending{t.Fatalf("checkout should remain pending: %#v",checkout)}
	if provider.initialized.AmountMinor!=6500000||provider.initialized.Reference!=checkout.Payment.Reference{t.Fatalf("provider received untrusted amount or reference: %#v",provider.initialized)}
	activated,err:=svc.VerifyCheckout(schoolID,checkout.Payment.Reference)
	if err!=nil{t.Fatal(err)}
	if activated.Payment.Status!=models.SubscriptionPaymentSucceeded||activated.Subscription.Status!=models.SubscriptionStatusActive{t.Fatalf("expected verified activation, got %#v",activated)}
	again,err:=svc.VerifyCheckout(schoolID,checkout.Payment.Reference)
	if err!=nil{t.Fatal(err)}
	if again.Subscription.ID!=activated.Subscription.ID{t.Fatal("repeated verification must return the same subscription")}
	var count int64
	if err:=db.Model(&models.SubscriptionEvent{}).Where("subscription_id = ? AND event_type = ?",""+activated.Subscription.ID.String(),"subscription_activated").Count(&count).Error;err!=nil{t.Fatal(err)}
	if count!=1{t.Fatalf("expected exactly one activation event, got %d",count)}
}

func TestSubscriptionCheckoutRejectsAmountMismatch(t *testing.T){
	db,schoolID,termID:=subscriptionBillingTestDB(t)
	now:=time.Date(2026,11,12,10,0,0,0,time.UTC)
	provider:=&subscriptionBillingFakeProvider{verification:PaymentVerification{AmountMinor:6500001,Currency:"NGN",Status:"succeeded",PaidAt:now}}
	svc:=NewSubscriptionBillingService(db,provider);svc.now=func()time.Time{return now}
	checkout,err:=svc.CreateCheckout(schoolID,termID,models.BillingPlanBasic,"admin@school.test","")
	if err!=nil{t.Fatal(err)}
	if _,err:=svc.VerifyCheckout(schoolID,checkout.Payment.Reference);!errors.Is(err,ErrSubscriptionPaymentInvalid){t.Fatalf("expected amount mismatch rejection, got %v",err)}
	var sub models.SchoolSubscription
	if err:=db.First(&sub,"id = ?",checkout.Subscription.ID).Error;err!=nil{t.Fatal(err)}
	if sub.Status!=models.SubscriptionStatusPending{t.Fatalf("mismatched amount must not activate subscription, got %q",sub.Status)}
}

func TestSubscriptionCheckoutInitializationFailureClosesPendingRecords(t *testing.T) {
	db,schoolID,termID:=subscriptionBillingTestDB(t)
	provider:=&subscriptionBillingFakeProvider{initializeErr:errors.New("provider unavailable")}
	svc:=NewSubscriptionBillingService(db,provider)
	_,err:=svc.CreateCheckout(schoolID,termID,models.BillingPlanBasic,"admin@school.test","")
	if !errors.Is(err,ErrSubscriptionPaymentRecoveryRequired){t.Fatalf("expected recoverable checkout state, got %v",err)}
	var payment models.SubscriptionPayment
	if err:=db.First(&payment).Error;err!=nil{t.Fatal(err)}
	if payment.Status!=models.SubscriptionPaymentPending{t.Fatalf("ambiguous initialization must remain pending for late-payment verification, got %q",payment.Status)}
	var sub models.SchoolSubscription
	if err:=db.First(&sub,"id = ?",payment.SubscriptionID).Error;err!=nil{t.Fatal(err)}
	if sub.Status!=models.SubscriptionStatusPending{t.Fatalf("ambiguous initialization must not cancel subscription, got %q",sub.Status)}
}

func TestSubscriptionCheckoutRetryReusesSavedAuthorizationURL(t *testing.T) {
	db, schoolID, termID := subscriptionBillingTestDB(t)
	provider := &subscriptionBillingFakeProvider{}
	svc := NewSubscriptionBillingService(db, provider)
	first, err := svc.CreateCheckout(schoolID, termID, models.BillingPlanBasic, "admin@school.test", "")
	if err != nil { t.Fatal(err) }
	second, err := svc.CreateCheckout(schoolID, termID, models.BillingPlanBasic, "admin@school.test", "")
	if err != nil { t.Fatal(err) }
	if second.Payment.ID != first.Payment.ID || second.Payment.Reference != first.Payment.Reference {
		t.Fatalf("retry must return the original pending payment: first=%#v second=%#v", first.Payment, second.Payment)
	}
	if second.Payment.AuthorizationURL != first.Payment.AuthorizationURL {
		t.Fatal("retry must return the original authorization URL")
	}
	if provider.initializeCalls != 1 {
		t.Fatalf("retry must not initialize a second provider transaction; calls=%d", provider.initializeCalls)
	}
}

func TestSubscriptionCheckoutRetryDoesNotReplaceDifferentPendingPlan(t *testing.T) {
	db, schoolID, termID := subscriptionBillingTestDB(t)
	provider := &subscriptionBillingFakeProvider{}
	svc := NewSubscriptionBillingService(db, provider)
	if _, err := svc.CreateCheckout(schoolID, termID, models.BillingPlanBasic, "admin@school.test", ""); err != nil { t.Fatal(err) }
	_, err := svc.CreateCheckout(schoolID, termID, models.BillingPlanProfessional, "admin@school.test", "")
	if !errors.Is(err, ErrSubscriptionPaymentPending) {
		t.Fatalf("expected pending checkout conflict, got %v", err)
	}
	if provider.initializeCalls != 1 { t.Fatalf("must not create a second provider transaction; calls=%d", provider.initializeCalls) }
}

func TestSubscriptionCheckoutMissingAuthorizationURLRemainsVerifiable(t *testing.T) {
	db, schoolID, termID := subscriptionBillingTestDB(t)
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	provider := &subscriptionBillingFakeProvider{verification: PaymentVerification{Provider:"paystack", AmountMinor:6500000, Currency:"NGN", Status:"succeeded", PaidAt:now}}
	svc := NewSubscriptionBillingService(db, provider)
	svc.now = func() time.Time { return now }
	checkout, err := svc.CreateCheckout(schoolID, termID, models.BillingPlanBasic, "admin@school.test", "")
	if err != nil { t.Fatal(err) }
	if err := db.Model(&models.SubscriptionPayment{}).Where("id = ?", checkout.Payment.ID).Update("authorization_url", "").Error; err != nil { t.Fatal(err) }
	_, err = svc.CreateCheckout(schoolID, termID, models.BillingPlanBasic, "admin@school.test", "")
	if !errors.Is(err, ErrSubscriptionPaymentRecoveryRequired) { t.Fatalf("expected recovery-required response, got %v", err) }
	activated, err := svc.VerifyCheckout(schoolID, checkout.Payment.Reference)
	if err != nil { t.Fatalf("payment must remain verifiable by its original reference: %v", err) }
	if activated.Payment.Status != models.SubscriptionPaymentSucceeded || activated.Subscription.Status != models.SubscriptionStatusActive {
		t.Fatalf("expected verified late payment to activate, got %#v", activated)
	}
}
