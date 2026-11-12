package database

import (
    "fmt"
    "os"
    "strings"

    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

// ConnectMigration opens the optional privileged connection used only by the
// versioned migration runner. A nil DB means the caller should retain the
// existing application connection for backwards compatibility.
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
