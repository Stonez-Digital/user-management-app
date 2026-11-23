package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type billingPostgresProvider struct {
	mu sync.Mutex
	initializeCalls int
	verifyCalls int
	verification PaymentVerification
}

func (p *billingPostgresProvider) Name() string { return "paystack" }

func (p *billingPostgresProvider) Initialize(_ context.Context, in PaystackInitializeInput) (PaystackInitializeResult, error) {
	p.mu.Lock()
	p.initializeCalls++
	p.mu.Unlock()
	return PaystackInitializeResult{
		AuthorizationURL: "https://checkout.paystack.com/" + in.Reference,
		AccessCode: "access-" + in.Reference,
		Reference: in.Reference,
	}, nil
}

func (p *billingPostgresProvider) Verify(reference string) (PaymentVerification, error) {
	p.mu.Lock()
	p.verifyCalls++
	p.mu.Unlock()
	result := p.verification
	result.Reference = reference
	return result, nil
}

func (p *billingPostgresProvider) counts() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.initializeCalls, p.verifyCalls
}

func subscriptionBillingPostgresTestDB(t *testing.T) (*gorm.DB, uuid.UUID, uuid.UUID) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL billing concurrency test skipped")
	}
	adminDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	schema := "subscription_billing_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := adminDB.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create isolated PostgreSQL test schema: %v", err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		_ = adminDB.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		_ = adminDB.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		t.Fatalf("open isolated PostgreSQL billing schema: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		_ = adminDB.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		if sqlDB, err := adminDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate isolated PostgreSQL schema: %v", err)
	}
	if err := db.AutoMigrate(
		&models.AcademicSession{}, &models.Term{}, &models.BillingPlan{},
		&models.SchoolSubscription{}, &models.SubscriptionEvent{}, &models.SubscriptionPayment{},
	); err != nil {
		t.Fatalf("migrate billing models: %v", err)
	}
	for _, statement := range []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS uq_test_school_one_pending ON school_subscriptions(school_id) WHERE status = 'pending_payment'",
		"CREATE UNIQUE INDEX IF NOT EXISTS uq_test_subscription_activation_once ON subscription_events(subscription_id) WHERE event_type = 'subscription_activated'",
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create billing invariant index: %v", err)
		}
	}

	school := models.School{
		Name: "Subscription Billing Concurrency Test",
		Code: "SUB-BILL-" + strings.ToUpper(uuid.NewString()[:8]),
		Status: models.SchoolStatusActive,
	}
	if err := db.Create(&school).Error; err != nil {
		t.Fatalf("create test school: %v", err)
	}
	now := time.Now().UTC()
	session := models.AcademicSession{
		SchoolID: school.ID, Name: "Billing Test Session " + uuid.NewString()[:8],
		StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(1, 0, 0),
		Status: models.AcademicStatusActive,
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create test academic session: %v", err)
	}
	term := models.Term{
		SchoolID: school.ID, AcademicSessionID: session.ID, Name: "Current Term",
		StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(1, 0, 0),
		Status: models.AcademicStatusActive,
	}
	if err := db.Create(&term).Error; err != nil {
		t.Fatalf("create test term: %v", err)
	}
	plan := models.BillingPlan{
		Code: models.BillingPlanBasic, Name: "Basic", AmountMinor: 6500000,
		Currency: "NGN", BillingInterval: "term", Active: true,
	}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("create test billing plan: %v", err)
	}
	return db, school.ID, term.ID
}

func TestSubscriptionBillingPostgresConcurrentCheckoutCreatesOnePayment(t *testing.T) {
	db, schoolID, termID := subscriptionBillingPostgresTestDB(t)
	provider := &billingPostgresProvider{}
	svc := NewSubscriptionBillingService(db, provider).WithTenantContext(schoolID, uuid.New())
	const callers = 10
	start := make(chan struct{})
	type outcome struct {
		checkout SubscriptionCheckout
		err error
	}
	results := make(chan outcome, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			checkout, err := svc.CreateCheckout(schoolID, termID, models.BillingPlanBasic, "billing@example.test", "")
			results <- outcome{checkout: checkout, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var paymentID uuid.UUID
	successes := 0
	for result := range results {
		if result.err != nil && !errors.Is(result.err, ErrSubscriptionPaymentRecoveryRequired) {
			t.Errorf("concurrent checkout returned unexpected error: %v", result.err)
			continue
		}
		if result.checkout.Payment.ID != uuid.Nil {
			if paymentID == uuid.Nil {
				paymentID = result.checkout.Payment.ID
			} else if result.checkout.Payment.ID != paymentID {
				t.Errorf("callers received different payment IDs: %s and %s", paymentID, result.checkout.Payment.ID)
			}
		}
		if result.err == nil {
			successes++
		}
	}
	if successes == 0 {
		t.Fatal("no caller received the initialized checkout")
	}
	var payments int64
	if err := db.Model(&models.SubscriptionPayment{}).Where("school_id = ?", schoolID).Count(&payments).Error; err != nil {
		t.Fatal(err)
	}
	if payments != 1 {
		t.Fatalf("expected one payment after concurrent checkout, got %d", payments)
	}
	var subscriptions int64
	if err := db.Model(&models.SchoolSubscription{}).Where("school_id = ? AND status = ?", schoolID, models.SubscriptionStatusPending).Count(&subscriptions).Error; err != nil {
		t.Fatal(err)
	}
	if subscriptions != 1 {
		t.Fatalf("expected one pending subscription, got %d", subscriptions)
	}
	initializeCalls, _ := provider.counts()
	if initializeCalls != 1 {
		t.Fatalf("expected exactly one provider initialization, got %d", initializeCalls)
	}
}

func TestSubscriptionBillingPostgresConcurrentVerificationActivatesOnce(t *testing.T) {
	db, schoolID, termID := subscriptionBillingPostgresTestDB(t)
	now := time.Now().UTC()
	provider := &billingPostgresProvider{
		verification: PaymentVerification{
			Provider: "paystack", AmountMinor: 6500000, Currency: "NGN",
			Status: "succeeded", PaidAt: now,
		},
	}
	svc := NewSubscriptionBillingService(db, provider).WithTenantContext(schoolID, uuid.New())
	checkout, err := svc.CreateCheckout(schoolID, termID, models.BillingPlanBasic, "billing@example.test", "")
	if err != nil {
		t.Fatalf("create checkout: %v", err)
	}

	const callers = 8
	start := make(chan struct{})
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.VerifyCheckout(schoolID, checkout.Payment.Reference)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent verification failed: %v", err)
		}
	}

	var payment models.SubscriptionPayment
	if err := db.Where("id = ?", checkout.Payment.ID).First(&payment).Error; err != nil {
		t.Fatal(err)
	}
	if payment.Status != models.SubscriptionPaymentSucceeded {
		t.Fatalf("expected successful payment, got %q", payment.Status)
	}
	var subscription models.SchoolSubscription
	if err := db.Where("id = ?", checkout.Subscription.ID).First(&subscription).Error; err != nil {
		t.Fatal(err)
	}
	if subscription.Status != models.SubscriptionStatusActive {
		t.Fatalf("expected active subscription, got %q", subscription.Status)
	}
	var events int64
	if err := db.Model(&models.SubscriptionEvent{}).
		Where("subscription_id = ? AND event_type = ?", subscription.ID, "subscription_activated").
		Count(&events).Error; err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Fatalf("expected exactly one activation event, got %d", events)
	}
}

func TestTenantTransactionPostgresClaimsAreTransactionLocal(t *testing.T) {
	db, _, _ := subscriptionBillingPostgresTestDB(t)
	schoolID, userID := uuid.New(), uuid.New()
	if err := tenantTransaction(db, schoolID, userID, func(tx *gorm.DB) error {
		var gotSchool, gotUser string
		if err := tx.Raw("SELECT current_setting('app.current_school_id', true), current_setting('app.current_user_id', true)").
			Row().Scan(&gotSchool, &gotUser); err != nil {
			return err
		}
		if gotSchool != schoolID.String() || gotUser != userID.String() {
			return fmt.Errorf("unexpected transaction claims: school=%q user=%q", gotSchool, gotUser)
		}
		return nil
	}); err != nil {
		t.Fatalf("read tenant claims in transaction: %v", err)
	}
	var schoolSetting, userSetting string
	if err := db.Raw("SELECT current_setting('app.current_school_id', true), current_setting('app.current_user_id', true)").
		Row().Scan(&schoolSetting, &userSetting); err != nil {
		t.Fatal(err)
	}
	if schoolSetting != "" || userSetting != "" {
		t.Fatalf("tenant claims leaked beyond transaction: school=%q user=%q", schoolSetting, userSetting)
	}
}
