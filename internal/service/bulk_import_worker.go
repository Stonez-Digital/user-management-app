package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const bulkImportPollInterval = 2 * time.Second
const bulkImportClaimLease = 5 * time.Minute

// withTenantTransaction gives every worker-owned transaction an explicit
// school and initiating-user context. PostgreSQL settings are transaction-local.
func (s *BulkImportService) withTenantTransaction(job models.BulkImportJob, fn func(*gorm.DB) error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT set_config('app.current_school_id', ?, true)", job.SchoolID.String()).Error; err != nil {
				return err
			}
			if err := tx.Exec("SELECT set_config('app.current_user_id', ?, true)", job.InitiatedBy.String()).Error; err != nil {
				return err
			}
		}
		return fn(tx)
	})
}

// claimNextJob is a deliberately system-level queue operation. The current
// deployment's postgres/BYPASSRLS connection can see the global queue; this
// must be replaced by a narrowly privileged queue role/service before a
// NOBYPASSRLS runtime cutover.
func (s *BulkImportService) claimNextJob() (*models.BulkImportJob, error) {
	var claimed *models.BulkImportJob
	cutoff := time.Now().UTC().Add(-bulkImportClaimLease)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var job models.BulkImportJob
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? OR (status = ? AND (claimed_at IS NULL OR claimed_at < ?))",
				models.BulkImportQueued, models.BulkImportRunning, cutoff).
			Order("created_at ASC").First(&job).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		workerID := s.workerID
		result := tx.Model(&models.BulkImportJob{}).
			Where("id = ? AND school_id = ? AND initiated_by = ?", job.ID, job.SchoolID, job.InitiatedBy).
			Updates(map[string]any{
				"status": models.BulkImportRunning,
				"worker_id": workerID,
				"claimed_at": now,
				"attempts": gorm.Expr("attempts + 1"),
				"last_error": "",
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		job.Status = models.BulkImportRunning
		job.WorkerID = &workerID
		job.ClaimedAt = &now
		job.Attempts++
		claimed = &job
		return nil
	})
	return claimed, err
}

func (s *BulkImportService) workerLoop() {
	for {
		job, err := s.claimNextJob()
		if err != nil {
			time.Sleep(bulkImportPollInterval)
			continue
		}
		if job == nil {
			time.Sleep(bulkImportPollInterval)
			continue
		}
		s.processClaimed(*job, s.workerID)
	}
}

func (s *BulkImportService) updateClaimedJob(job models.BulkImportJob, workerID uuid.UUID, values map[string]any) error {
	return s.withTenantTransaction(job, func(tx *gorm.DB) error {
		query := tx.Model(&models.BulkImportJob{}).
			Where("id = ? AND school_id = ? AND initiated_by = ?", job.ID, job.SchoolID, job.InitiatedBy)
		if workerID != uuid.Nil {
			query = query.Where("worker_id = ?", workerID)
		}
		result := query.Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("bulk import worker lease was lost")
		}
		return nil
	})
}

func (s *BulkImportService) processClaimed(job models.BulkImportJob, workerID uuid.UUID) {
	fail := func(err error) {
		message := err.Error()
		values := map[string]any{
			"status": models.BulkImportFailed,
			"last_error": message,
			"errors_json": mustJSON([]BulkError{{Message: message}}),
			"worker_id": nil,
			"claimed_at": nil,
		}
		_ = s.updateClaimedJob(job, workerID, values)
	}

	err := s.withTenantTransaction(job, func(tx *gorm.DB) error {
		if workerID != uuid.Nil {
			var initiator models.User
			if err := tx.Where("id = ? AND school_id = ?", job.InitiatedBy, job.SchoolID).First(&initiator).Error; err != nil {
				return fmt.Errorf("initiating user is not a member of the job's school: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		fail(err)
		return
	}

	var rows []BulkRow
	if err := json.Unmarshal([]byte(job.RowsJSON), &rows); err != nil {
		fail(fmt.Errorf("invalid stored import payload: %w", err))
		return
	}
	var errs []BulkError
	if strings.TrimSpace(job.ErrorsJSON) != "" {
		if err := json.Unmarshal([]byte(job.ErrorsJSON), &errs); err != nil {
			fail(fmt.Errorf("invalid stored import errors: %w", err))
			return
		}
	}
	validRows := map[int]bool{}
	for _, item := range errs {
		validRows[item.Row] = true
	}

	createdCount, skippedCount, failedCount := 0, 0, 0
	credentialsSent, credentialFailures := 0, 0
	for index, row := range rows {
		rowNumber := index + 2
		if validRows[rowNumber] {
			continue
		}
		created, skipped, email, password, role, rowErr := s.processRow(job, row)
		if skipped {
			skippedCount++
		}
		if rowErr != nil {
			failedCount++
			errs = append(errs, BulkError{Row: rowNumber, Message: rowErr.Error()})
		} else {
			if created {
				createdCount++
			}
			if job.SendCredentials && created {
				if s.mailer == nil {
					credentialFailures++
					errs = append(errs, BulkError{Row: rowNumber, Field: "email", Message: "credential mailer is unavailable"})
				} else if sendErr := s.mailer.SendCredentials(strings.TrimSpace(row["name"]), email, role, password); sendErr != nil {
					credentialFailures++
					errs = append(errs, BulkError{Row: rowNumber, Field: "email", Message: "account created but credential email failed: " + sendErr.Error()})
				} else {
					credentialsSent++
				}
			}
		}
		values := map[string]any{
			"created": createdCount, "skipped": skippedCount, "failed": failedCount,
			"credentials_sent": credentialsSent, "credential_emails_failed": credentialFailures,
			"errors_json": mustJSON(errs), "claimed_at": time.Now().UTC(),
		}
		if err := s.updateClaimedJob(job, workerID, values); err != nil {
			return
		}
	}
	status := models.BulkImportCompleted
	if failedCount > 0 || len(errs) > 0 || credentialFailures > 0 {
		status = models.BulkImportCompletedWithErrors
	}
	_ = s.updateClaimedJob(job, workerID, map[string]any{
		"status": status, "errors_json": mustJSON(errs),
		"created": createdCount, "skipped": skippedCount, "failed": failedCount,
		"credentials_sent": credentialsSent, "credential_emails_failed": credentialFailures,
		"worker_id": nil, "claimed_at": nil, "last_error": "",
	})
}

// run remains as a deterministic entry point for service tests. Production
// execution always comes through the database-backed claim loop.
func (s *BulkImportService) run(job models.BulkImportJob) {
	s.processClaimed(job, uuid.Nil)
}
