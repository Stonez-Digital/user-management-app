package service

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// openBulkWorkerPostgres creates a private schema per test so migration and
// queue state cannot collide with other packages running against TEST_DATABASE_URL.
func openBulkWorkerPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect PostgreSQL admin database: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	schema := "bulk_worker_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		_ = adminSQL.Close()
		t.Fatalf("create isolated test schema: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		_ = adminSQL.Close()
	})

	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect isolated PostgreSQL schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate isolated PostgreSQL schema: %v", err)
	}
	return db
}

func TestBulkWorkerPostgresConcurrentClaimsSkipLockedJobs(t *testing.T) {
	db := openBulkWorkerPostgres(t)
	schoolID, actor := uuid.New(), uuid.New()
	first := models.BulkImportJob{
		SchoolID: schoolID, InitiatedBy: actor, Kind: "students",
		FileName: "first.csv", Status: models.BulkImportQueued,
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	second := models.BulkImportJob{
		SchoolID: schoolID, InitiatedBy: actor, Kind: "students",
		FileName: "second.csv", Status: models.BulkImportQueued,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first queued job: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second queued job: %v", err)
	}

	// Hold the oldest job's row lock as if another worker is claiming it.
	// A concurrent claimant must skip that row and claim the second job.
	lockTx := db.Begin()
	if lockTx.Error != nil {
		t.Fatalf("begin lock transaction: %v", lockTx.Error)
	}
	var locked models.BulkImportJob
	if err := lockTx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", first.ID).First(&locked).Error; err != nil {
		_ = lockTx.Rollback().Error
		t.Fatalf("lock oldest queued job: %v", err)
	}

	workerA := NewBulkImportService(db, nil)
	workerB := NewBulkImportService(db, nil)
	type claimResult struct {
		job *models.BulkImportJob
		err error
	}
	resultCh := make(chan claimResult, 1)
	go func() {
		job, err := workerA.claimNextJob()
		resultCh <- claimResult{job: job, err: err}
	}()

	var firstClaim claimResult
	select {
	case firstClaim = <-resultCh:
	case <-time.After(5 * time.Second):
		_ = lockTx.Rollback().Error
		t.Fatal("claim blocked on the locked row; PostgreSQL SKIP LOCKED was not applied")
	}
	if firstClaim.err != nil {
		_ = lockTx.Rollback().Error
		t.Fatalf("first concurrent claim: %v", firstClaim.err)
	}
	if firstClaim.job == nil || firstClaim.job.ID != second.ID {
		_ = lockTx.Rollback().Error
		t.Fatalf("expected worker A to skip locked oldest job and claim %s, got %+v", second.ID, firstClaim.job)
	}

	if err := lockTx.Commit().Error; err != nil {
		t.Fatalf("release oldest job lock: %v", err)
	}
	secondClaim, err := workerB.claimNextJob()
	if err != nil {
		t.Fatalf("second worker claim after lock release: %v", err)
	}
	if secondClaim == nil || secondClaim.ID != first.ID {
		t.Fatalf("expected worker B to claim released oldest job %s, got %+v", first.ID, secondClaim)
	}
	if firstClaim.job.ID == secondClaim.ID {
		t.Fatal("two workers claimed the same job")
	}
	if firstClaim.job.WorkerID == nil || *firstClaim.job.WorkerID != workerA.workerID {
		t.Fatal("first job does not record worker A ownership")
	}
	if secondClaim.WorkerID == nil || *secondClaim.WorkerID != workerB.workerID {
		t.Fatal("second job does not record worker B ownership")
	}

	var claimed int64
	if err := db.Model(&models.BulkImportJob{}).
		Where("status = ? AND worker_id IN ?", models.BulkImportRunning, []uuid.UUID{workerA.workerID, workerB.workerID}).
		Count(&claimed).Error; err != nil {
		t.Fatalf("count claimed jobs: %v", err)
	}
	if claimed != 2 {
		t.Fatalf("expected two distinct running claims, got %d (%s)", claimed, fmt.Sprint(claimed))
	}
}
