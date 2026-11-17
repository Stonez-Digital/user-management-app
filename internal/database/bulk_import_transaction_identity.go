package database

import (
	"fmt"
	"strconv"

	"gorm.io/gorm"
)

// BulkImportTransactionIdentity is read from the active PostgreSQL transaction
// and included in the HMAC claims to prevent assertion transfer to another
// backend session or transaction.
type BulkImportTransactionIdentity struct {
	BackendPID    int32
	TransactionID uint64
}

func CurrentBulkImportTransactionIdentity(tx *gorm.DB) (BulkImportTransactionIdentity, error) {
	if tx == nil || tx.Dialector.Name() != "postgres" {
		return BulkImportTransactionIdentity{}, fmt.Errorf("bulk import transaction identity requires PostgreSQL")
	}
	var raw struct {
		BackendPID    int32  `gorm:"column:backend_pid"`
		TransactionID string `gorm:"column:transaction_id"`
	}
	if err := tx.Raw("SELECT pg_backend_pid() AS backend_pid, pg_current_xact_id()::text AS transaction_id").Scan(&raw).Error; err != nil {
		return BulkImportTransactionIdentity{}, fmt.Errorf("read PostgreSQL transaction identity: %w", err)
	}
	transactionID, err := strconv.ParseUint(raw.TransactionID, 10, 64)
	if err != nil || raw.BackendPID <= 0 || transactionID == 0 {
		return BulkImportTransactionIdentity{}, fmt.Errorf("PostgreSQL returned an invalid transaction identity")
	}
	return BulkImportTransactionIdentity{BackendPID: raw.BackendPID, TransactionID: transactionID}, nil
}
