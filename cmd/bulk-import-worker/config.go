package main

import (
	"fmt"
	"strings"
)

// requiredWorkerDatabaseURLs returns the two mandatory, independent worker
// capabilities. It deliberately does not read or fall back to DATABASE_URL.
func requiredWorkerDatabaseURLs(getenv func(string) string) (processingDSN, claimDSN string, err error) {
	if getenv == nil {
		return "", "", fmt.Errorf("bulk import worker environment reader is required")
	}
	processingDSN = strings.TrimSpace(getenv("BULK_IMPORT_PROCESSING_DATABASE_URL"))
	if processingDSN == "" {
		return "", "", fmt.Errorf("bulk import worker requires BULK_IMPORT_PROCESSING_DATABASE_URL")
	}
	claimDSN = strings.TrimSpace(getenv("BULK_IMPORT_CLAIM_DATABASE_URL"))
	if claimDSN == "" {
		return "", "", fmt.Errorf("bulk import worker requires BULK_IMPORT_CLAIM_DATABASE_URL")
	}
	if processingDSN == claimDSN {
		return "", "", fmt.Errorf("bulk import processing and claim database URLs must be distinct")
	}
	return processingDSN, claimDSN, nil
}
