package service

import (
	"errors"
	"net/url"
	"os"
	"sync"
	"testing"
	"strings"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// These tests use the disposable PostgreSQL service configured by Production QA.
// They deliberately skip local runs that do not provide TEST_DATABASE_URL.
func careerPathPostgresTestService(t *testing.T) (*gorm.DB, *CareerPathService, uuid.UUID, uuid.UUID) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL concurrency test skipped")
	}
	adminDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	schema := "careerpath_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
		t.Fatalf("open isolated PostgreSQL test schema: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate isolated PostgreSQL test schema: %v", err)
	}

	var school models.School
	if err := db.Where("code = ?", "DEFAULT").First(&school).Error; err != nil {
		t.Fatalf("load default test school: %v", err)
	}
	user := models.User{
		SchoolID: &school.ID,
		Name:     "CareerPath concurrency test",
		Email:    "careerpath-concurrency-" + uuid.NewString() + "@example.test",
		Role:     "student",
		Active:   true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create synthetic test user: %v", err)
	}
	student := models.Student{
		SchoolID:        school.ID,
		UserID:           user.ID,
		AdmissionNumber: "CP-" + uuid.NewString(),
	}
	if err := db.Create(&student).Error; err != nil {
		_ = db.Delete(&user).Error
		t.Fatalf("create synthetic test student: %v", err)
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
	return db, NewCareerPathService(db), school.ID, user.ID
}

func TestCareerPathPostgresConcurrentCreateOrResumeKeepsOneDraft(t *testing.T) {
	db, svc, schoolID, userID := careerPathPostgresTestService(t)
	const workers = 12
	start := make(chan struct{})
	type outcome struct {
		attempt CareerPathAttemptView
		err     error
	}
	outcomes := make(chan outcome, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			attempt, err := svc.CreateOrResume(schoolID, userID)
			outcomes <- outcome{attempt: attempt, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(outcomes)

	var winningID uuid.UUID
	for result := range outcomes {
		if result.err != nil {
			t.Errorf("concurrent create/resume failed: %v", result.err)
			continue
		}
		if winningID == uuid.Nil {
			winningID = result.attempt.ID
		} else if result.attempt.ID != winningID {
			t.Errorf("concurrent callers received different drafts: %s and %s", winningID, result.attempt.ID)
		}
	}
	if winningID == uuid.Nil {
		t.Fatal("no concurrent caller returned a draft")
	}
	var drafts int64
	if err := db.Model(&models.CareerPathAttempt{}).
		Where("school_id = ? AND user_id = ? AND version = ? AND status = ?", schoolID, userID, CareerPathAssessmentVersion, "in_progress").
		Count(&drafts).Error; err != nil {
		t.Fatalf("count drafts: %v", err)
	}
	if drafts != 1 {
		t.Fatalf("expected exactly one draft after concurrent create/resume, got %d", drafts)
	}
}

func TestCareerPathPostgresConcurrentSaveAndSubmitNeverLosesSubmittedState(t *testing.T) {
	_, svc, schoolID, userID := careerPathPostgresTestService(t)
	attempt, err := svc.CreateOrResume(schoolID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveResponses(schoolID, userID, attempt.ID, completeCareerPathResponses()); err != nil {
		t.Fatalf("prepare complete draft: %v", err)
	}

	start := make(chan struct{})
	type outcome struct {
		kind string
		err  error
	}
	outcomes := make(chan outcome, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, err := svc.SaveResponses(schoolID, userID, attempt.ID, map[string][]string{"interests": {"build", "create"}})
		outcomes <- outcome{kind: "save", err: err}
	}()
	go func() {
		defer wg.Done()
		_, err := svc.Submit(schoolID, userID, attempt.ID)
		outcomes <- outcome{kind: "submit", err: err}
	}()
	close(start)
	wg.Wait()
	close(outcomes)

	submitNeedsRetry := false
	for result := range outcomes {
		switch result.kind {
		case "save":
			if result.err != nil &&
				!errors.Is(result.err, ErrCareerPathConflict) &&
				!errors.Is(result.err, ErrCareerPathSubmitted) {
				t.Errorf("concurrent save returned unexpected error: %v", result.err)
			}
		case "submit":
			if errors.Is(result.err, ErrCareerPathConflict) {
				submitNeedsRetry = true
			} else if result.err != nil {
				t.Errorf("concurrent submit returned unexpected error: %v", result.err)
			}
		}
	}
	if submitNeedsRetry {
		if _, err := svc.Submit(schoolID, userID, attempt.ID); err != nil {
			t.Fatalf("retry submit after an explicit optimistic-lock conflict: %v", err)
		}
	}

	final, err := svc.Get(schoolID, userID, attempt.ID)
	if err != nil {
		t.Fatalf("reload attempt after concurrent operations: %v", err)
	}
	if final.Status != "submitted" || len(final.Results) == 0 {
		t.Fatalf("expected persisted submitted state and results, got status=%q results=%d", final.Status, len(final.Results))
	}
	if _, err := svc.SaveResponses(schoolID, userID, attempt.ID, completeCareerPathResponses()); !errors.Is(err, ErrCareerPathSubmitted) {
		t.Fatalf("submitted attempt must remain immutable after concurrent operations; got %v", err)
	}
}

func TestCareerPathPostgresConcurrentSavesLeaveValidResponses(t *testing.T) {
	_, svc, schoolID, userID := careerPathPostgresTestService(t)
	attempt, err := svc.CreateOrResume(schoolID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveResponses(schoolID, userID, attempt.ID, completeCareerPathResponses()); err != nil {
		t.Fatalf("prepare draft: %v", err)
	}

	start := make(chan struct{})
	values := []string{"build", "create"}
	type outcome struct{ err error }
	outcomes := make(chan outcome, len(values))
	var wg sync.WaitGroup
	for _, value := range values {
		value := value
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.SaveResponses(schoolID, userID, attempt.ID, map[string][]string{"interests": {value}})
			outcomes <- outcome{err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(outcomes)
	for result := range outcomes {
		if result.err != nil && !errors.Is(result.err, ErrCareerPathConflict) && !errors.Is(result.err, ErrCareerPathSubmitted) {
			t.Errorf("concurrent save returned unexpected error: %v", result.err)
		}
	}

	final, err := svc.Get(schoolID, userID, attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "in_progress" {
		t.Fatalf("concurrent saves must not submit the draft, got status %q", final.Status)
	}
	selected := final.Responses["interests"]
	if len(selected) != 1 || (selected[0] != "build" && selected[0] != "create") {
		t.Fatalf("concurrent saves left invalid or mixed interests response: %#v", selected)
	}
}
