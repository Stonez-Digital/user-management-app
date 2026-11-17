package database

import (
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openSignedContextTestDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open signed-context test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestBulkImportSignedContextReadinessAndProcessingRole(t *testing.T) {
	runnerDSN := os.Getenv("BULK_CONTEXT_TEST_DATABASE_URL")
	workerDSN := os.Getenv("BULK_CONTEXT_TEST_WORKER_URL")
	if runnerDSN == "" || workerDSN == "" {
		t.Skip("disposable PostgreSQL signed-context test URLs are not set")
	}

	runnerDB := openSignedContextTestDB(t, runnerDSN)
	if err := ValidateBulkImportProcessingRole(runnerDB); err == nil {
		t.Fatal("privileged database owner was accepted as the processing login")
	}

	workerDB := openSignedContextTestDB(t, workerDSN)
	if err := ValidateBulkImportProcessingRole(workerDB); err != nil {
		t.Fatalf("dedicated processing login rejected: %v", err)
	}
	if err := ValidateBulkImportSignedContextReady(workerDB); err != nil {
		t.Fatalf("processing login cannot inspect verifier readiness: %v", err)
	}
}
