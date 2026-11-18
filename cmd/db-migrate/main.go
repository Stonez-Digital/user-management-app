package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/onoja217/users-management-app/internal/database"
)

type migrationConfig struct {
	dsn              string
	expectedHost     string
	expectedDatabase string
}

func loadMigrationConfig() (migrationConfig, error) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("DB_DRIVER")), "postgres") {
		return migrationConfig{}, fmt.Errorf("DB_DRIVER must be postgres")
	}

	cfg := migrationConfig{
		dsn:              strings.TrimSpace(os.Getenv("MIGRATION_DATABASE_URL")),
		expectedHost:     strings.TrimSpace(os.Getenv("MIGRATION_EXPECTED_HOST")),
		expectedDatabase: strings.TrimSpace(os.Getenv("MIGRATION_EXPECTED_DATABASE")),
	}
	if cfg.dsn == "" {
		return migrationConfig{}, fmt.Errorf("MIGRATION_DATABASE_URL is required")
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("MIGRATION_TARGET")), "staging") {
		return migrationConfig{}, fmt.Errorf("MIGRATION_TARGET must explicitly be staging")
	}
	if cfg.expectedHost == "" || cfg.expectedDatabase == "" {
		return migrationConfig{}, fmt.Errorf("MIGRATION_EXPECTED_HOST and MIGRATION_EXPECTED_DATABASE are required")
	}

	parsed, err := url.Parse(cfg.dsn)
	if err != nil {
		return migrationConfig{}, fmt.Errorf("parse migration database URL: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return migrationConfig{}, fmt.Errorf("MIGRATION_DATABASE_URL must use the postgres or postgresql URL scheme")
	}
	if parsed.Hostname() == "" || !strings.EqualFold(parsed.Hostname(), cfg.expectedHost) {
		return migrationConfig{}, fmt.Errorf("migration database host does not match MIGRATION_EXPECTED_HOST")
	}
	if parsed.Query().Get("sslmode") == "disable" {
		return migrationConfig{}, fmt.Errorf("TLS must not be disabled for staging migrations")
	}

	return cfg, nil
}

func run() error {
	cfg, err := loadMigrationConfig()
	if err != nil {
		return err
	}

	// ConnectMigration reads only MIGRATION_DATABASE_URL. The ordinary
	// DATABASE_URL is deliberately not used by this migration-only command.
	db, err := database.ConnectMigration()
	if err != nil {
		return err
	}
	if db == nil {
		return fmt.Errorf("migration database connection was not created")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get migration database connection: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("ping migration database: %w", err)
	}

	var target struct {
		Database string
		Host     string
		Role     string
	}
	if err := db.Raw("SELECT current_database() AS database, inet_server_addr()::text AS host, current_user AS role").Scan(&target).Error; err != nil {
		return fmt.Errorf("verify migration database identity: %w", err)
	}
	if target.Database != cfg.expectedDatabase {
		return fmt.Errorf("connected database %q does not match MIGRATION_EXPECTED_DATABASE", target.Database)
	}
	log.Printf("running application migrations against approved staging database %q as role %q", target.Database, target.Role)

	if err := database.Migrate(db); err != nil {
		return fmt.Errorf("application migrations failed: %w", err)
	}
	log.Printf("application migrations completed successfully for staging database %q", target.Database)
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Printf("migration command failed: %v", err)
		os.Exit(1)
	}
}
