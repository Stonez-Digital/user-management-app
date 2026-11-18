package main

import (
	"testing"
)

func setValidStagingEnvironment(t *testing.T, dsn string) {
	t.Helper()
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("MIGRATION_TARGET", "staging")
	t.Setenv("MIGRATION_DATABASE_URL", dsn)
}

func TestLoadMigrationConfigRequiresExplicitStagingTarget(t *testing.T) {
	setValidStagingEnvironment(t, "postgres://migrator:secret@"+approvedStagingHost+"/"+approvedStagingDatabase+"?sslmode=require")
	t.Setenv("MIGRATION_TARGET", "")

	if _, err := loadMigrationConfig(); err == nil {
		t.Fatal("expected missing staging target to be rejected")
	}
}

func TestLoadMigrationConfigRejectsNonStagingHost(t *testing.T) {
	setValidStagingEnvironment(t, "postgres://migrator:secret@db.example.com/"+approvedStagingDatabase+"?sslmode=require")

	if _, err := loadMigrationConfig(); err == nil {
		t.Fatal("expected non-staging host to be rejected")
	}
}

func TestLoadMigrationConfigRejectsWrongDatabase(t *testing.T) {
	setValidStagingEnvironment(t, "postgres://migrator:secret@"+approvedStagingHost+"/production?sslmode=require")

	if _, err := loadMigrationConfig(); err == nil {
		t.Fatal("expected wrong database to be rejected")
	}
}

func TestLoadMigrationConfigRejectsTLSDisabled(t *testing.T) {
	setValidStagingEnvironment(t, "postgres://migrator:secret@"+approvedStagingHost+"/"+approvedStagingDatabase+"?sslmode=disable")

	if _, err := loadMigrationConfig(); err == nil {
		t.Fatal("expected sslmode=disable to be rejected")
	}
}

func TestLoadMigrationConfigAcceptsApprovedStagingURL(t *testing.T) {
	setValidStagingEnvironment(t, "postgres://migrator:secret@"+approvedStagingHost+"/"+approvedStagingDatabase+"?sslmode=require")

	if _, err := loadMigrationConfig(); err != nil {
		t.Fatalf("loadMigrationConfig() error = %v", err)
	}
}
