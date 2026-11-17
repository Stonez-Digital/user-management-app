package database

import (
	"fmt"

	"github.com/onoja217/users-management-app/internal/security/bulkcontext"
	"gorm.io/gorm"
)

// VerifyBulkImportContext invokes the protected PostgreSQL verifier inside the
// caller's transaction. The caller must use the same tx handle for every
// tenant read/write authorized by this assertion.
func VerifyBulkImportContext(tx *gorm.DB, assertion bulkcontext.Assertion) error {
	if tx == nil || tx.Dialector.Name() != "postgres" {
		return fmt.Errorf("signed bulk import context requires a PostgreSQL transaction")
	}
	claims := assertion.Claims
	if assertion.Signature == "" {
		return fmt.Errorf("signed bulk import context signature is required")
	}
	if err := tx.Exec(
		"SELECT stonez_internal.verify_bulk_import_context(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		claims.KeyID,
		claims.AssertionID,
		claims.JobID,
		claims.SchoolID,
		claims.InitiatedBy,
		claims.WorkerID,
		claims.BackendPID,
		claims.TransactionID,
		claims.LeaseGeneration,
		claims.IssuedAt,
		claims.ExpiresAt,
		assertion.Signature,
	).Error; err != nil {
		return fmt.Errorf("verify signed bulk import tenant context: %w", err)
	}
	return nil
}
