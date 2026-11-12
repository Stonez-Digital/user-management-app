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
}
func (p *subscriptionBillingFakeProvider) Name() string { return "paystack" }
func (p *subscriptionBillingFakeProvider) Initialize(_ context.Context, in PaystackInitializeInput) (PaystackInitializeResult,error) {
	p.initialized=in
	if p.initializeErr!=nil{return PaystackInitializeResult{},p.initializeErr}
	return PaystackInitializeResult{AuthorizationURL:"https://checkout.paystack.com/example",AccessCode:"access-example",Reference:in.Reference},nil
}
func (p *subscriptionBillingFakeProvider) Verify(reference string)(PaymentVerification,error){
	if p.verifyErr!=nil{return PaymentVerification{},p.verifyErr}
	v:=p.verification;v.Reference=reference;return v,nil
}

func subscriptionBillingTestDB(t *testing.T)(*gorm.DB,uuid.UUID,uuid.UUID){
	t.Helper()
	db,err:=gorm.Open(sqlite.Open("file:subscription-billing-test?mode=memory&cache=shared"),&gorm.Config{})
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
