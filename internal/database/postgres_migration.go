package database

import (
    "fmt"
    "os"
    "strings"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

// ValidateMigrationDatabaseConfiguration prevents a restricted runtime role
// from being reused for schema migrations or one-time platform bootstrap.
// A separate migration credential is mandatory when the RLS startup guard is
// enabled; callers may retain the legacy fallback while the guard is disabled.
func ValidateMigrationDatabaseConfiguration(requireRestrictedRuntime bool, migrationDSN string) error {
    if requireRestrictedRuntime && strings.TrimSpace(migrationDSN) == "" {
        return fmt.Errorf("REQUIRE_NOBYPASSRLS requires MIGRATION_DATABASE_URL so migrations and platform bootstrap use a separate privileged connection")
    }
    return nil
}

// ConnectMigration opens the optional privileged connection used only by the
// versioned migration runner. A nil DB means the caller should retain the
// existing application connection for backwards compatibility when the
// restricted runtime guard is disabled.
func ConnectMigration() (*gorm.DB, error) {
    dsn := strings.TrimSpace(os.Getenv("MIGRATION_DATABASE_URL"))
    if dsn == "" {
        return nil, nil
    }
    if strings.TrimSpace(os.Getenv("DB_DRIVER")) != "postgres" {
        return nil, fmt.Errorf("MIGRATION_DATABASE_URL requires DB_DRIVER=postgres")
    }

    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        return nil, fmt.Errorf("connect migration postgres database: %w", err)
    }
    return db, nil
}
