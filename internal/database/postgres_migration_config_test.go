package database

import (
    "strings"
    "testing"
)

func TestValidateMigrationDatabaseConfiguration(t *testing.T) {
    tests := []struct {
        name string
        restricted bool
        migrationDSN string
        wantErr bool
    }{
        {name: "legacy mode permits fallback", restricted: false, migrationDSN: "", wantErr: false},
        {name: "legacy mode permits separate migration database", restricted: false, migrationDSN: "postgres://migration", wantErr: false},
        {name: "restricted mode requires separate migration database", restricted: true, migrationDSN: "", wantErr: true},
        {name: "restricted mode rejects whitespace migration database", restricted: true, migrationDSN: "  ", wantErr: true},
        {name: "restricted mode accepts separate migration database", restricted: true, migrationDSN: "postgres://migration", wantErr: false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateMigrationDatabaseConfiguration(tt.restricted, tt.migrationDSN)
            if (err != nil) != tt.wantErr {
                t.Fatalf("ValidateMigrationDatabaseConfiguration() error = %v, wantErr %t", err, tt.wantErr)
            }
            if err != nil && !strings.Contains(err.Error(), "MIGRATION_DATABASE_URL") {
                t.Fatalf("expected actionable migration credential error, got %v", err)
            }
        })
    }
}
