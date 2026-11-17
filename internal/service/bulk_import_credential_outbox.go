package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

type accountSetupNotification struct {
	EventID       uuid.UUID
	UserID        uuid.UUID
	RecipientEmail string
	DisplayName   string
	AccountRole   string
	DeliveryLease uuid.UUID
	AttemptCount  int
}

type accountSetupNotificationCounts struct {
	DeliveredCount int
	FailedCount    int
	PendingCount   int
}

// enqueueAccountSetupNotification must be called inside the same signed
// transaction that creates the imported account and profile.
func (s *BulkImportService) enqueueAccountSetupNotification(tx *gorm.DB, userID uuid.UUID, email, name, role string) error {
	if tx == nil || tx.Dialector.Name() != "postgres" {
		return errors.New("durable account-setup notifications require PostgreSQL")
	}
	if userID == uuid.Nil {
		return errors.New("account-setup notification requires a user ID")
	}
	var eventID uuid.UUID
	row := tx.Raw(
		"SELECT stonez_context_private.enqueue_account_setup_notification(?, ?, ?, ?)",
		userID, email, name, role,
	).Row()
	if err := row.Scan(&eventID); err != nil {
		return fmt.Errorf("enqueue durable account-setup notification: %w", err)
	}
	if eventID == uuid.Nil {
		return errors.New("database did not return an account-setup notification ID")
	}
	return nil
}

func (s *BulkImportService) claimAccountSetupNotification(job models.BulkImportJob) (accountSetupNotification, bool, error) {
	var notification accountSetupNotification
	found := false
	err := s.withTenantTransaction(job, func(tx *gorm.DB) error {
		result := tx.Raw("SELECT * FROM stonez_context_private.claim_account_setup_notification()").Scan(&notification)
		if result.Error != nil {
			return fmt.Errorf("claim account-setup notification: %w", result.Error)
		}
		found = result.RowsAffected == 1
		return nil
	})
	return notification, found, err
}

func (s *BulkImportService) completeAccountSetupNotification(job models.BulkImportJob, notification accountSetupNotification) (bool, error) {
	var completed bool
	err := s.withTenantTransaction(job, func(tx *gorm.DB) error {
		result := tx.Raw(
			"SELECT stonez_context_private.complete_account_setup_notification(?, ?, NULL)",
			notification.EventID, notification.DeliveryLease,
		).Scan(&completed)
		return result.Error
	})
	return completed, err
}

func (s *BulkImportService) retryAccountSetupNotification(job models.BulkImportJob, notification accountSetupNotification, failureCode string) error {
	delay := accountSetupRetryDelay(notification.AttemptCount)
	return s.withTenantTransaction(job, func(tx *gorm.DB) error {
		var status string
		result := tx.Raw(
			"SELECT stonez_context_private.retry_account_setup_notification(?, ?, ?, ?)",
			notification.EventID, notification.DeliveryLease, failureCode, int(delay.Seconds()),
		).Scan(&status)
		if result.Error != nil { return result.Error }
		if status == "stale_lease" || status == "" {
			return errors.New("account-setup notification delivery lease was lost")
		}
		return nil
	})
}

func (s *BulkImportService) accountSetupNotificationCounts(job models.BulkImportJob) (accountSetupNotificationCounts, error) {
	var counts accountSetupNotificationCounts
	err := s.withTenantTransaction(job, func(tx *gorm.DB) error {
		return tx.Raw("SELECT * FROM stonez_context_private.account_setup_notification_counts()").Scan(&counts).Error
	})
	return counts, err
}

func accountSetupRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := 30 * time.Second
	for i := 1; i < attempt && delay < time.Hour; i++ {
		delay *= 2
	}
	if delay > time.Hour {
		delay = time.Hour
	}
	return delay
}

// deliverAccountSetupNotifications sends outside database transactions. A
// provider failure is recorded against the exact delivery lease and leaves the
// job running so the queue can reclaim it after a worker crash/restart.
func (s *BulkImportService) deliverAccountSetupNotifications(job models.BulkImportJob) (accountSetupNotificationCounts, error) {
	for {
		notification, found, err := s.claimAccountSetupNotification(job)
		if err != nil {
			return accountSetupNotificationCounts{}, err
		}
		if !found {
			break
		}
		if s.mailer == nil || !s.mailer.Enabled() {
			if err := s.retryAccountSetupNotification(job, notification, "mailer_unconfigured"); err != nil {
				return accountSetupNotificationCounts{}, err
			}
			continue
		}
		if err := s.mailer.SendAccountSetupNotice(
			notification.DisplayName,
			notification.RecipientEmail,
			notification.AccountRole,
			notification.EventID.String(),
		); err != nil {
			if retryErr := s.retryAccountSetupNotification(job, notification, "provider_rejected"); retryErr != nil {
				return accountSetupNotificationCounts{}, retryErr
			}
			continue
		}
		completed, err := s.completeAccountSetupNotification(job, notification)
		if err != nil {
			return accountSetupNotificationCounts{}, err
		}
		if !completed {
			// The lease expired or was reclaimed while the provider request was
			// in flight. Do not let this worker overwrite the newer lease.
			break
		}
	}
	return s.accountSetupNotificationCounts(job)
}
