package database

import (
	"fmt"

	"gorm.io/gorm"
)

// ValidateBulkImportContextVerifier is a read-only startup preflight. It does
// not inspect or return signing-key material and does not mutate PostgreSQL.
func ValidateBulkImportContextVerifier(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return fmt.Errorf("signed bulk import context requires PostgreSQL")
	}
	var status struct {
		VerifierExists bool `gorm:"column:verifier_exists"`
		ContextExists  bool `gorm:"column:context_exists"`
		HMACExists     bool `gorm:"column:hmac_exists"`
		CanVerify      bool `gorm:"column:can_verify"`
	}
	err := db.Raw(`
		SELECT
			to_regprocedure('stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,bigint,bigint,bigint,text)') IS NOT NULL AS verifier_exists,
			to_regprocedure('stonez_internal.current_bulk_import_context()') IS NOT NULL AS context_exists,
			to_regprocedure('extensions.hmac(bytea,bytea,text)') IS NOT NULL AS hmac_exists,
			has_function_privilege(current_user, 'stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,bigint,bigint,bigint,text)', 'EXECUTE') AS can_verify
	`).Scan(&status).Error
	if err != nil {
		return fmt.Errorf("inspect signed bulk import context verifier: %w", err)
	}
	if !status.VerifierExists || !status.ContextExists || !status.HMACExists || !status.CanVerify {
		return fmt.Errorf("signed bulk import context verifier is not installed or executable (verifier=%t context=%t pgcrypto_hmac=%t execute=%t)",
			status.VerifierExists, status.ContextExists, status.HMACExists, status.CanVerify)
	}
	return nil
}
