package main

import (
	"net/url"
	"testing"
)

func TestLoadMigrationConfigRequiresExplicitStagingTarget(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("MIGRATION_DATABASE_URL", "postgres://migrator:secret@ep-example.eu-central-1.aws.neon.tech/stonez_staging?sslmode=require")
	t.Setenv("MIGRATION_EXPECTED_HOST", "ep-example.eu-central-1.aws.neon.tech")
	t.Setenv("MIGRATION_EXPECTED_DATABASE", "stonez_staging")
	t.Setenv("MIGRATION_TARGET", "")

	if _, err := loadMigrationConfig(); err == nil {
		t.Fatal("expected missing staging target to be rejected")
	}
}

func TestLoadMigrationConfigRejectsWrongHost(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("MIGRATION_DATABASE_URL", "postgres://migrator:secret@db.example.com/stonez_staging?sslmode=require")
	t.Setenv("MIGRATION_EXPECTED_HOST", "ep-example.eu-central-1.aws.neon.tech")
	t.Setenv("MIGRATION_EXPECTED_DATABASE", "stonez_staging")
	t.Setenv("MIGRATION_TARGET", "staging")

	if _, err := loadMigrationConfig(); err == nil {
		t.Fatal("expected host mismatch to be rejected")
	}
}

func TestLoadMigrationConfigRejectsTLSDisabled(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("MIGRATION_DATABASE_URL", "postgres://migrator:secret@ep-example.eu-central-1.aws.neon.tech/stonez_staging?sslmode=disable")
	t.Setenv("MIGRATION_EXPECTED_HOST", "ep-example.eu-central-1.aws.neon.tech")
	t.Setenv("MIGRATION_EXPECTED_DATABASE", "stonez_staging")
	t.Setenv("MIGRATION_TARGET", "staging")

	if _, err := loadMigrationConfig(); err == nil {
		t.Fatal("expected sslmode=disable to be rejected")
	}
}

func TestLoadMigrationConfigAcceptsMatchingStagingURL(t *testing.T) {
	const host = "ep-example.eu-central-1.aws.neon.tech"
	dsn := "postgres://migrator:secret@" + host + "/stonez_staging?sslmode=require"
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Hostname() != host {
		t.Fatalf("parsed host = %q, want %q", parsed.Hostname(), host)
	}

	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("MIGRATION_DATABASE_URL", dsn)
	t.Setenv("MIGRATION_EXPECTED_HOST", host)
	t.Setenv("MIGRATION_EXPECTED_DATABASE", "stonez_staging")
	t.Setenv("MIGRATION_TARGET", "staging")

	cfg, err := loadMigrationConfig()
	if err != nil {
		t.Fatalf("loadMigrationConfig() error = %v", err)
	}
	if cfg.expectedDatabase != "stonez_staging" {
		t.Fatalf("expected database = %q, want stonez_staging", cfg.expectedDatabase)
	}
}
