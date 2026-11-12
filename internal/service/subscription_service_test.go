package service

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func subscriptionTestDB(t *testing.T) (*gorm.DB, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.AcademicSession{}, &models.Term{},
		&models.BillingPlan{}, &models.SchoolSubscription{}, &models.SubscriptionEvent{}); err != nil {
		t.Fatal(err)
	}
	schoolID := uuid.New()
	school := models.School{ID: schoolID, Name: "Trial School", Code: "TRIAL-01", Status: models.SchoolStatusActive}
	if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
	now := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	session := models.AcademicSession{SchoolID: schoolID, Name: "2026/2027", StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(0, 9, 0), Status: models.AcademicStatusActive}
	if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	term := models.Term{SchoolID: schoolID, AcademicSessionID: session.ID, Name: "First", StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(0, 3, 0), Status: models.AcademicStatusActive}
	if err := db.Create(&term).Error; err != nil { t.Fatal(err) }
	trial := models.BillingPlan{Code: models.BillingPlanTrial, Name: "Free Trial", Currency: "NGN", BillingInterval: "term", Active: true}
	if err := db.Create(&trial).Error; err != nil { t.Fatal(err) }
	return db, schoolID, term.ID, trial.ID
}

func TestStartTrialUsesSchoolTermEndAndRecordsEvent(t *testing.T) {
	db, schoolID, termID, _ := subscriptionTestDB(t)
	now := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	svc := NewSubscriptionServiceWithClock(db, func() time.Time { return now })
	sub, err := svc.StartTrial(schoolID, termID)
	if err != nil { t.Fatal(err) }
	if !sub.IsTrial || sub.Status != models.SubscriptionStatusActive || sub.AmountMinor != 0 {
		t.Fatalf("unexpected trial state: %+v", sub)
	}
	if !sub.ExpiresAt.Equal(now.AddDate(0, 3, 0)) {
		t.Fatalf("trial must end with the selected academic term; got %s", sub.ExpiresAt)
	}
	var events int64
	if err := db.Model(&models.SubscriptionEvent{}).Where("subscription_id = ? AND event_type = ?", sub.ID, models.SubscriptionEventTrialStarted).Count(&events).Error; err != nil { t.Fatal(err) }
	if events != 1 { t.Fatalf("expected one trial-start event, got %d", events) }
}

func TestStartTrialCannotBeReused(t *testing.T) {
	db, schoolID, termID, _ := subscriptionTestDB(t)
	svc := NewSubscriptionServiceWithClock(db, func() time.Time { return time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC) })
	if _, err := svc.StartTrial(schoolID, termID); err != nil { t.Fatal(err) }
	if _, err := svc.StartTrial(schoolID, termID); !errors.Is(err, ErrSubscriptionTrialUsed) {
		t.Fatalf("expected trial reuse rejection, got %v", err)
	}
}

func TestStartTrialRejectsAnotherSchoolsTerm(t *testing.T) {
	db, schoolID, _, _ := subscriptionTestDB(t)
	otherSchool := models.School{ID: uuid.New(), Name: "Other School", Code: "OTHER-01", Status: models.SchoolStatusActive}
	if err := db.Create(&otherSchool).Error; err != nil { t.Fatal(err) }
	now := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	session := models.AcademicSession{SchoolID: otherSchool.ID, Name: "2026/2027", StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(0, 9, 0), Status: models.AcademicStatusActive}
	if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	term := models.Term{SchoolID: otherSchool.ID, AcademicSessionID: session.ID, Name: "First", StartDate: now.AddDate(0, -1, 0), EndDate: now.AddDate(0, 3, 0), Status: models.AcademicStatusActive}
	if err := db.Create(&term).Error; err != nil { t.Fatal(err) }
	svc := NewSubscriptionServiceWithClock(db, func() time.Time { return now })
	if _, err := svc.StartTrial(schoolID, term.ID); !errors.Is(err, ErrSubscriptionTermUnavailable) {
		t.Fatalf("expected tenant-scoped term rejection, got %v", err)
	}
}

func TestCurrentSubscriptionExpiresAndAudits(t *testing.T) {
	db, schoolID, termID, planID := subscriptionTestDB(t)
	now := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	sub := models.SchoolSubscription{SchoolID: schoolID, PlanID: planID, TermID: termID, Status: models.SubscriptionStatusActive, IsTrial: true, StartsAt: now.AddDate(0, -1, 0), ExpiresAt: now}
	if err := db.Create(&sub).Error; err != nil { t.Fatal(err) }
	svc := NewSubscriptionServiceWithClock(db, func() time.Time { return now })
	if _, err := svc.CurrentSubscription(schoolID); !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("expected expired subscription to be unavailable, got %v", err)
	}
	var stored models.SchoolSubscription
	if err := db.First(&stored, "id = ?", sub.ID).Error; err != nil { t.Fatal(err) }
	if stored.Status != models.SubscriptionStatusExpired { t.Fatalf("expected expired status, got %s", stored.Status) }
	var events int64
	if err := db.Model(&models.SubscriptionEvent{}).Where("subscription_id = ? AND event_type = ?", sub.ID, models.SubscriptionEventExpired).Count(&events).Error; err != nil { t.Fatal(err) }
	if events != 1 { t.Fatalf("expected one expiry event, got %d", events) }
}

func TestCheckQuotaUsesPlanLimit(t *testing.T) {
	db, schoolID, termID, planID := subscriptionTestDB(t)
	now := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	limit := 50
	if err := db.Model(&models.BillingPlan{}).Where("id = ?", planID).Update("max_students", limit).Error; err != nil { t.Fatal(err) }
	sub := models.SchoolSubscription{SchoolID: schoolID, PlanID: planID, TermID: termID, Status: models.SubscriptionStatusActive, IsTrial: true, StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}
	if err := db.Create(&sub).Error; err != nil { t.Fatal(err) }
	svc := NewSubscriptionServiceWithClock(db, func() time.Time { return now })
	if err := svc.CheckQuota(schoolID, "students", 49); err != nil { t.Fatalf("49 of 50 students should be allowed: %v", err) }
	if err := svc.CheckQuota(schoolID, "students", 50); !errors.Is(err, ErrSubscriptionQuotaExceeded) {
		t.Fatalf("expected quota rejection at 50, got %v", err)
	}
}

func TestStartTrialRequiresApprovedSchool(t *testing.T) {
	db, schoolID, termID, _ := subscriptionTestDB(t)
	if err := db.Model(&models.School{}).Where("id = ?", schoolID).Update("status", models.SchoolStatusPending).Error; err != nil {
		t.Fatal(err)
	}
	svc := NewSubscriptionServiceWithClock(db, func() time.Time {
		return time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	})
	if _, err := svc.StartTrial(schoolID, termID); !errors.Is(err, ErrSubscriptionInvalid) {
		t.Fatalf("expected pending school rejection, got %v", err)
	}
}
