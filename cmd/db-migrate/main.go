package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/onoja217/users-management-app/internal/database"
)

// These identifiers are pinned to the approved, isolated Neon staging project.
// Do not make them environment-overridable: that would let a production URL
// pass by simply changing the expected-host variables.
const (
	approvedStagingHost     = "ep-snowy-term-b94cw7vl.c-7.eu-central-1.aws.neon.tech"
	approvedStagingDatabase = "stonez_staging"
)

type migrationConfig struct {
	dsn string
}

func loadMigrationConfig() (migrationConfig, error) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("DB_DRIVER")), "postgres") {
		return migrationConfig{}, fmt.Errorf("DB_DRIVER must be postgres")
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("MIGRATION_TARGET")), "staging") {
		return migrationConfig{}, fmt.Errorf("MIGRATION_TARGET must explicitly be staging")
	}

	dsn := strings.TrimSpace(os.Getenv("MIGRATION_DATABASE_URL"))
	if dsn == "" {
		return migrationConfig{}, fmt.Errorf("MIGRATION_DATABASE_URL is required")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		return migrationConfig{}, fmt.Errorf("parse migration database URL: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return migrationConfig{}, fmt.Errorf("MIGRATION_DATABASE_URL must use the postgres or postgresql URL scheme")
	}
	if !strings.EqualFold(parsed.Hostname(), approvedStagingHost) {
		return migrationConfig{}, fmt.Errorf("migration database host is not the approved Neon staging endpoint")
	}
	if parsed.Path != "/"+approvedStagingDatabase {
		return migrationConfig{}, fmt.Errorf("migration database name is not the approved staging database")
	}
	switch parsed.Query().Get("sslmode") {
	case "require", "verify-ca", "verify-full":
		// Accepted TLS modes for the isolated staging connection.
	default:
		return migrationConfig{}, fmt.Errorf("MIGRATION_DATABASE_URL must set sslmode=require, verify-ca, or verify-full")
	}

	return migrationConfig{dsn: dsn}, nil
}

func run() error {
	if _, err := loadMigrationConfig(); err != nil {
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
		Role     string
	}
	if err := db.Raw("SELECT current_database() AS database, current_user AS role").Scan(&target).Error; err != nil {
		return fmt.Errorf("verify migration database identity: %w", err)
	}
	if target.Database != approvedStagingDatabase {
		return fmt.Errorf("connected database %q is not the approved staging database", target.Database)
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
