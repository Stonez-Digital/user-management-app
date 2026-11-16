package main

import (
	"context"
	"errors"
	"log"
	"database/sql"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/service"
)

func main() {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("DB_DRIVER")), "postgres") {
		log.Fatal("bulk import worker requires DB_DRIVER=postgres")
	}
	if strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		log.Fatal("bulk import worker requires DATABASE_URL for job processing")
	}
	claimDSN := strings.TrimSpace(os.Getenv("BULK_IMPORT_CLAIM_DATABASE_URL"))
	if claimDSN == "" {
		log.Fatal("bulk import worker requires BULK_IMPORT_CLAIM_DATABASE_URL")
	}

	// DATABASE_URL is used only for the existing tenant-scoped job processing
	// operations. The claim-only credential is opened separately and is never
	// used for application table reads or writes.
	processingDB, err := database.Connect()
	if err != nil {
		log.Fatalf("connect bulk import processing database: %v", err)
	}
	claimDB, err := database.ConnectPostgresURL(claimDSN)
	if err != nil {
		log.Fatalf("connect restricted bulk import claim database: %v", err)
	}
	if err := database.ValidateBulkImportClaimRole(claimDB); err != nil {
		log.Fatalf("restricted bulk import claim role validation failed: %v", err)
	}

	role, err := database.InspectPostgresRole(processingDB)
	if err != nil {
		log.Fatalf("inspect bulk import processing database role: %v", err)
	}
	if role != nil && (role.SessionSuperuser || role.SessionBypassRLS || role.SessionCanAssumePrivilegedRole) {
		log.Printf("SECURITY WARNING: bulk import processing database session %q is privileged; tenant GUCs are not a substitute for a restricted processing role", role.SessionRole)
	}

	worker, err := service.NewBulkImportServiceWithClaimDB(processingDB, claimDB, service.NewCredentialMailerFromEnv())
	if err != nil {
		log.Fatalf("configure bulk import worker: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log.Printf("bulk import worker started with separate restricted claim connection")
	err = worker.RunWorker(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("bulk import worker stopped: %v", err)
	}
	log.Printf("bulk import worker stopped")
	closeDatabase(processingDB, "processing")
	closeDatabase(claimDB, "claim")
}

func closeDatabase(db interface{ DB() (*sql.DB, error) }, name string) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("close %s database: get connection pool: %v", name, err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		log.Printf("close %s database: %v", name, err)
	}
}
