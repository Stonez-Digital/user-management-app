package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/service"
	"github.com/onoja217/users-management-app/internal/security/bulkcontext"
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

	processingRole, err := database.InspectBulkImportProcessingRole(processingDB)
	if err != nil {
		log.Fatalf("inspect bulk import processing database role: %v", err)
	}
	if err := database.ValidateBulkImportProcessingRole(processingRole); err != nil {
		log.Fatal(err)
	}
	if err := database.ValidateBulkImportContextVerifier(processingDB); err != nil {
		log.Fatal(err)
	}

	vaultAddress := strings.TrimSpace(os.Getenv("BULK_IMPORT_CONTEXT_VAULT_ADDR"))
	vaultTokenFile := strings.TrimSpace(os.Getenv("BULK_IMPORT_CONTEXT_VAULT_TOKEN_FILE"))
	vaultMount := strings.TrimSpace(os.Getenv("BULK_IMPORT_CONTEXT_VAULT_KV_MOUNT"))
	vaultPath := strings.TrimSpace(os.Getenv("BULK_IMPORT_CONTEXT_VAULT_KV_PATH"))
	if vaultAddress == "" || vaultTokenFile == "" || vaultMount == "" || vaultPath == "" {
		log.Fatal("bulk import worker requires BULK_IMPORT_CONTEXT_VAULT_ADDR, BULK_IMPORT_CONTEXT_VAULT_TOKEN_FILE, BULK_IMPORT_CONTEXT_VAULT_KV_MOUNT, and BULK_IMPORT_CONTEXT_VAULT_KV_PATH")
	}
	keySource, err := bulkcontext.NewVaultKVKeySource(vaultAddress, vaultTokenFile, vaultMount, vaultPath, nil, 30*time.Second)
	if err != nil {
		log.Fatalf("configure Vault bulk import key source: %v", err)
	}
	activeContextKey, err := keySource.ActiveSigningKey(context.Background())
	if err != nil {
		log.Fatalf("load active bulk import context key from Vault: %v", err)
	}
	if err := database.ValidateBulkImportContextKey(processingDB, activeContextKey.ID); err != nil {
		log.Fatal(err)
	}
	for i := range activeContextKey.Secret {
		activeContextKey.Secret[i] = 0
	}
	contextSigner, err := bulkcontext.NewSigner(keySource, 2*time.Minute)
	if err != nil {
		log.Fatalf("configure bulk import context signer: %v", err)
	}

	worker, err := service.NewBulkImportServiceWithClaimDB(processingDB, claimDB, service.NewCredentialMailerFromEnv())
	if err != nil {
		log.Fatalf("configure bulk import worker: %v", err)
	}
	worker = worker.WithBulkContextSigner(contextSigner)

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
