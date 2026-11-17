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
	KeyCheckExists bool `gorm:"column:key_check_exists"
	CanCheckKey    bool `gorm:"column:can_check_key"`
	}
	err := db.Raw(`
		SELECT
			to_regprocedure('stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,bigint,bigint,bigint,text)') IS NOT NULL AS verifier_exists,
			to_regprocedure('stonez_internal.current_bulk_import_context()') IS NOT NULL AS context_exists,
			to_regprocedure('extensions.hmac(bytea,bytea,text)') IS NOT NULL AS hmac_exists,
			has_function_privilege(current_user, 'stonez_internal.verify_bulk_import_context(text,uuid,uuid,uuid,uuid,uuid,bigint,bigint,bigint,text)', 'EXECUTE') AS can_verify,
			to_regprocedure('stonez_internal.bulk_import_context_key_available(text)') IS NOT NULL AS key_check_exists,
			has_function_privilege(current_user, 'stonez_internal.bulk_import_context_key_available(text)', 'EXECUTE') AS can_check_key
	`).Scan(&status).Error
	if err != nil {
		return fmt.Errorf("inspect signed bulk import context verifier: %w", err)
	}
	if !status.VerifierExists || !status.ContextExists || !status.HMACExists || !status.CanVerify || !status.KeyCheckExists || !status.CanCheckKey {
		return fmt.Errorf("signed bulk import context verifier is not installed or executable (verifier=%t context=%t pgcrypto_hmac=%t execute=%t)",
			status.VerifierExists, status.ContextExists, status.HMACExists, status.CanVerify)
	}
	return nil
}

 
// ValidateBulkImportContextKey confirms that the active Vault key has been
// provisioned into the private PostgreSQL verifier table without exposing it.
func ValidateBulkImportContextKey(db *gorm.DB, keyID string) error {
	if db == nil || db.Dialector.Name() != "postgres" || keyID == "" {
		return fmt.Errorf("a PostgreSQL connection and active bulk context key ID are required")
	}
	var available bool
	if err := db.Raw("SELECT stonez_internal.bulk_import_context_key_available(?)", keyID).Scan(&available).Error; err != nil {
		return fmt.Errorf("check active bulk import context key availability: %w", err)
	}
	if !available {
		return fmt.Errorf("active Vault bulk import context key %q is not provisioned in the PostgreSQL verifier", keyID)
	}
	return nil
}
