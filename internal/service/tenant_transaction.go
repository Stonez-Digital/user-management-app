package service

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// tenantTransaction keeps the school and actor claims transaction-local.
// Provider/network calls must never be made from its callback.
func tenantTransaction(db *gorm.DB, schoolID, userID uuid.UUID, fn func(*gorm.DB) error) error {
	if db == nil || fn == nil {
		return errors.New("tenant transaction database and callback are required")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if schoolID != uuid.Nil || userID != uuid.Nil {
			if schoolID == uuid.Nil || userID == uuid.Nil {
				return errors.New("complete tenant and user context is required")
			}
			if tx.Dialector.Name() == "postgres" {
				if err := tx.Exec(
					"SELECT set_config('app.current_school_id', ?, true), set_config('app.current_user_id', ?, true)",
					schoolID.String(), userID.String(),
				).Error; err != nil {
					return err
				}
			}
		}
		return fn(tx)
	})
}
