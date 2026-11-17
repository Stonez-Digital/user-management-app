package main

import (
	"context"
	"crypto/hmac"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/onoja217/users-management-app/internal/security/bulkcontext"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bulk context key provisioning failed:", err)
		os.Exit(1)
	}
}

func run() error {
	vaultAddress := requiredEnv("BULK_IMPORT_CONTEXT_VAULT_ADDR")
	vaultTokenFile := requiredEnv("BULK_IMPORT_CONTEXT_VAULT_TOKEN_FILE")
	vaultMount := requiredEnv("BULK_IMPORT_CONTEXT_VAULT_KV_MOUNT")
	vaultPath := requiredEnv("BULK_IMPORT_CONTEXT_VAULT_KV_PATH")
	databaseURL := requiredEnv("BULK_IMPORT_CONTEXT_PROVISIONER_DATABASE_URL")

	parsed, err := url.Parse(databaseURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return errors.New("provisioner database URL must use the PostgreSQL scheme")
	}
	sslMode := strings.ToLower(parsed.Query().Get("sslmode"))
	if sslMode != "require" && sslMode != "verify-ca" && sslMode != "verify-full" {
		return errors.New("provisioner database URL must require TLS using sslmode=require, verify-ca, or verify-full")
	}

	keySource, err := bulkcontext.NewVaultKVKeySource(
		vaultAddress, vaultTokenFile, vaultMount, vaultPath, nil, 30*time.Second,
	)
	if err != nil {
		return fmt.Errorf("configure Vault key source: %w", err)
	}
	key, err := keySource.ActiveSigningKey(context.Background())
	if err != nil {
		return fmt.Errorf("read active signing key from Vault: %w", err)
	}
	defer func() {
		for i := range key.Secret {
			key.Secret[i] = 0
		}
	}()

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return errors.New("connect to the PostgreSQL verifier database")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return errors.New("open PostgreSQL verifier database connection")
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return errors.New("PostgreSQL verifier database is unreachable")
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []byte
		scanErr := tx.Raw(
			"SELECT secret FROM stonez_internal.bulk_context_keys WHERE key_id = ?",
			key.ID,
		).Row().Scan(&existing)
		switch {
		case scanErr == nil:
			defer func() {
				for i := range existing {
					existing[i] = 0
				}
			}()
			if !hmac.Equal(existing, key.Secret) {
				return fmt.Errorf("key ID %q already exists with different verifier material; use a new immutable key ID", key.ID)
			}
			return tx.Exec(
				"UPDATE stonez_internal.bulk_context_keys SET enabled = true WHERE key_id = ?",
				key.ID,
			).Error
		case errors.Is(scanErr, sql.ErrNoRows):
			return tx.Exec(
				"INSERT INTO stonez_internal.bulk_context_keys (key_id, secret, enabled) VALUES (?, ?, true)",
				key.ID, key.Secret,
			).Error
		default:
			return errors.New("read existing verifier key row")
		}
	})
	if err != nil {
		return fmt.Errorf("store active Vault key in private PostgreSQL verifier table: %w", err)
	}

	fmt.Printf("Provisioned active bulk-import context key ID %q. Key material was not displayed.\n", key.ID)
	return nil
}

func requiredEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		fmt.Fprintln(os.Stderr, "required environment variable is missing:", name)
		os.Exit(2)
	}
	return value
}
