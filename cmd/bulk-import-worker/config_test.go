package main

import (
	"strings"
	"testing"
)

func TestRequiredWorkerDatabaseURLs(t *testing.T) {
	tests := []struct {
		name string
		env map[string]string
		wantErr string
	}{
		{name: "both dedicated credentials", env: map[string]string{
			"BULK_IMPORT_PROCESSING_DATABASE_URL": "postgres://processing/db",
			"BULK_IMPORT_CLAIM_DATABASE_URL": "postgres://claim/db",
		}},
		{name: "processing credential required", env: map[string]string{
			"BULK_IMPORT_CLAIM_DATABASE_URL": "postgres://claim/db",
		}, wantErr: "BULK_IMPORT_PROCESSING_DATABASE_URL"},
		{name: "claim credential required", env: map[string]string{
			"BULK_IMPORT_PROCESSING_DATABASE_URL": "postgres://processing/db",
		}, wantErr: "BULK_IMPORT_CLAIM_DATABASE_URL"},
		{name: "shared credential rejected", env: map[string]string{
			"BULK_IMPORT_PROCESSING_DATABASE_URL": "postgres://same/db",
			"BULK_IMPORT_CLAIM_DATABASE_URL": "postgres://same/db",
		}, wantErr: "must be distinct"},
		{name: "API database URL is not a fallback", env: map[string]string{
			"DATABASE_URL": "postgres://api/db",
		}, wantErr: "BULK_IMPORT_PROCESSING_DATABASE_URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(key string) string { return tt.env[key] }
			processing, claim, err := requiredWorkerDatabaseURLs(getenv)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if processing != "postgres://processing/db" || claim != "postgres://claim/db" {
				t.Fatalf("unexpected URLs: processing=%q claim=%q", processing, claim)
			}
		})
	}
}

func TestRequiredWorkerDatabaseURLsRejectNilReader(t *testing.T) {
	if _, _, err := requiredWorkerDatabaseURLs(nil); err == nil {
		t.Fatal("expected nil environment reader to fail")
	}
}
