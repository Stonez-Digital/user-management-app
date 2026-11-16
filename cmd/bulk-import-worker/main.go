package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

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

	requireProcessingRole, err := parseOptionalBoolEnv("REQUIRE_BULK_IMPORT_PROCESS_ROLE")
	if err != nil {
		log.Fatal(err)
	}
	role, err := database.InspectBulkImportProcessingRole(processingDB)
	if err != nil {
		log.Fatalf("inspect bulk import processing database role: %v", err)
	}
	if err := database.ValidateBulkImportProcessingRole(role, requireProcessingRole); err != nil {
		log.Fatal(err)
	}
	if role != nil && !requireProcessingRole &&
		(!role.SessionCanLogin || !role.CurrentRoleMatchesSession || role.CurrentRole != role.SessionRole ||
			role.CurrentSuperuser || role.CurrentBypassRLS || role.SessionSuperuser || role.SessionBypassRLS ||
			role.SessionCanCreateDB || role.SessionCanCreateRole || role.SessionCanAssumePrivilegedRole ||
			role.SessionIsAPIRuntimeMember || role.SessionIsQueueOwnerMember || role.SessionIsServiceRoleMember) {
		log.Printf("SECURITY WARNING: bulk import processing database session %q is privileged or belongs to a shared runtime role; tenant GUCs are not a substitute for a restricted processing role", role.SessionRole)
	}

	worker, err := service.NewBulkImportServiceWithClaimDB(processingDB, claimDB, service.NewCredentialMailerFromEnv())
	if err != nil {
		log.Fatalf("configure bulk import worker: %v", err)
	}

	maxRuntime := 4 * time.Minute
	if configured := strings.TrimSpace(os.Getenv("BULK_IMPORT_WORKER_MAX_RUNTIME")); configured != "" {
		parsed, parseErr := time.ParseDuration(configured)
		if parseErr != nil || parsed <= 0 || parsed > 4*time.Minute+30*time.Second {
			log.Fatal("BULK_IMPORT_WORKER_MAX_RUNTIME must be a valid duration greater than zero and no longer than 4m30s")
		}
		maxRuntime = parsed
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workerCtx, cancel := context.WithTimeout(ctx, maxRuntime)
	defer cancel()
	log.Printf("bulk import worker started with separate restricted claim connection; polling_window=%s", maxRuntime)
	err = worker.RunWorker(workerCtx)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		log.Fatalf("bulk import worker stopped: %v", err)
	}
	log.Printf("bulk import worker polling window ended")
	closeDatabase(processingDB, "processing")
	closeDatabase(claimDB, "claim")
}

func parseOptionalBoolEnv(name string) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.New(name + " must be true or false")
	}
	return parsed, nil
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
