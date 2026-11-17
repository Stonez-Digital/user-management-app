package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
 "github.com/onoja217/users-management-app/internal/security/bulkimportcontext"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
 "gorm.io/gorm/logger"
)

const bulkImportPollInterval = 2 * time.Second
const bulkImportClaimLease = 5 * time.Minute
const bulkImportIdleTimeout = 30 * time.Second

// withTenantTransaction gives every worker-owned transaction an explicit
// school and initiating-user context. PostgreSQL settings are transaction-local.
func (s *BulkImportService) withTenantTransaction(job models.BulkImportJob, fn func(*gorm.DB) error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if s.requireSignedContext {
				if s.assertionSigner == nil || job.WorkerID == nil || job.LeaseID == nil || *job.LeaseID == uuid.Nil {
					return errors.New("signed bulk worker context is required but signer or lease identity is missing")
				}
				assertion, err := s.assertionSigner.Sign(bulkimportcontext.Context{
					JobID: job.ID, SchoolID: job.SchoolID, InitiatedBy: job.InitiatedBy,
					WorkerID: *job.WorkerID, LeaseID: *job.LeaseID,
				})
				if err != nil {
					return fmt.Errorf("sign bulk worker transaction context: %w", err)
				}
				// Assertions are short-lived bearer credentials. Suppress ORM query
				// logging for this call so the token is never interpolated into logs.
				verifierTx := tx.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
				if err := verifierTx.Exec("SELECT stonez_context_private.verify_bulk_import_context(?)", assertion).Error; err != nil {
					return fmt.Errorf("verify bulk worker transaction context failed: %w", err)
				}
			} else {
				// Legacy mode remains for the existing worker login until the
				// dedicated processing role and every required worker RLS policy
				// have been installed and verified.
				if err := tx.Exec("SELECT set_config('app.current_school_id', ?, true)", job.SchoolID.String()).Error; err != nil {
					return err
				}
				if err := tx.Exec("SELECT set_config('app.current_user_id', ?, true)", job.InitiatedBy.String()).Error; err != nil {
					return err
				}
			}
		}

		// Fence every worker-owned transaction against an expired or replaced
		// lease. Locking the job row prevents another worker from reclaiming it
		// while this row/job transaction is in flight, even if it runs longer
		// than the lease interval. The next transaction must still present the
		// current worker ID and a non-expired lease.
		if job.WorkerID != nil {
			if job.LeaseID == nil || *job.LeaseID == uuid.Nil {
				return errors.New("bulk import worker lease generation is missing")
			}
			var lease models.BulkImportJob
			leaseQuery := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND school_id = ? AND initiated_by = ? AND worker_id = ? AND status = ?",
					job.ID, job.SchoolID, job.InitiatedBy, *job.WorkerID, models.BulkImportRunning)
			// The lease generation changes on every claim/reclaim. Include it
			// in the locked predicate so replacement cannot race verification.
			leaseQuery = leaseQuery.Where("lease_id = ?", *job.LeaseID)
			err := leaseQuery.First(&lease).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("bulk import worker lease was lost or expired")
				}
				return err
			}
			// Check expiry only after acquiring the row lock. A timestamp
			// captured before a lock wait could otherwise authorize a lease
			// that expired while this transaction was blocked.
			leaseClock := time.Now().UTC()
			if tx.Dialector.Name() == "postgres" {
				if err := tx.Raw("SELECT clock_timestamp()").Scan(&leaseClock).Error; err != nil {
					return err
				}
			}
			if lease.ClaimedAt == nil || lease.ClaimedAt.Before(leaseClock.Add(-bulkImportClaimLease)) {
				return errors.New("bulk import worker lease was lost or expired")
			}
		}
		return fn(tx)
	})
}

// claimNextJob uses a narrowly granted SECURITY DEFINER function on PostgreSQL.
// There is deliberately no direct-table fallback for PostgreSQL: a missing or
// misconfigured function is a hard worker error, not a reason to use BYPASSRLS.
// SQLite retains the direct transactional implementation for local unit tests.
func (s *BulkImportService) claimNextJob() (*models.BulkImportJob, error) {
	var claimed *models.BulkImportJob
	claimDB := s.claimDB
	if claimDB == nil {
		return nil, errors.New("bulk import claim database is not configured")
	}
	if claimDB.Dialector.Name() == "postgres" {
		err := claimDB.Transaction(func(tx *gorm.DB) error {
			var job models.BulkImportJob
			result := tx.Raw("SELECT * FROM stonez_internal.claim_next_bulk_import_job(?)", s.workerID).Scan(&job)
			if result.Error != nil {
				return fmt.Errorf("claim bulk import job through restricted queue function: %w", result.Error)
			}
			if result.RowsAffected == 0 {
				return nil
			}
			claimed = &job
			return nil
		})
		return claimed, err
	}

	cutoff := time.Now().UTC().Add(-bulkImportClaimLease)
	err := claimDB.Transaction(func(tx *gorm.DB) error {
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
		leaseID := uuid.New()
		result := tx.Model(&models.BulkImportJob{}).
			Where("id = ? AND school_id = ? AND initiated_by = ?", job.ID, job.SchoolID, job.InitiatedBy).
			Updates(map[string]any{
				"status": models.BulkImportRunning,
				"worker_id": workerID,
				"lease_id": leaseID,
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
		job.LeaseID = &leaseID
		job.ClaimedAt = &now
		job.Attempts++
		claimed = &job
		return nil
	})
	return claimed, err
}

func (s *BulkImportService) workerLoop() {
	_ = s.RunWorker(context.Background())
}

// RunWorker polls the claim-only connection and processes returned jobs using
// the application database connection. It exits promptly when ctx is cancelled.
func (s *BulkImportService) RunWorker(ctx context.Context) error {
	if s.db == nil || s.claimDB == nil {
		return errors.New("bulk import worker requires application and claim database connections")
	}
	// Bind both database handles to the worker lifetime so cancellation and
	// the scheduled-run deadline interrupt in-flight database operations, not
	// only the polling loop between jobs.
	worker := &BulkImportService{
		db:        s.db.WithContext(ctx),
		claimDB:   s.claimDB.WithContext(ctx),
		mailer:    s.mailer,
		workerID:  s.workerID,
		workerOnce: s.workerOnce,
		assertionSigner: s.assertionSigner,
		requireSignedContext: s.requireSignedContext,
	}

	lastWorkAt := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		job, err := worker.claimNextJob()
		if err != nil {
			log.Printf("bulk import worker claim failed: %v", err)
			if !waitForWorkerPoll(ctx) { return ctx.Err() }
			continue
		}
		if job == nil {
			if time.Since(lastWorkAt) >= bulkImportIdleTimeout {
				return nil
			}
			if !waitForWorkerPoll(ctx) { return ctx.Err() }
			continue
		}
		worker.processClaimed(*job, worker.workerID)
		if err := ctx.Err(); err != nil {
			return err
		}
		lastWorkAt = time.Now()
	}
}

func waitForWorkerPoll(ctx context.Context) bool {
	timer := time.NewTimer(bulkImportPollInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *BulkImportService) updateClaimedJob(job models.BulkImportJob, workerID uuid.UUID, values map[string]any) error {
	return s.withTenantTransaction(job, func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if claimedAt, ok := values["claimed_at"]; ok && claimedAt != nil {
				values["claimed_at"] = gorm.Expr("clock_timestamp()")
			}
		}
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
			if err := tx.Select("id", "school_id").Where("id = ? AND school_id = ?", job.InitiatedBy, job.SchoolID).First(&initiator).Error; err != nil {
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
	// Recompute immutable upload validation errors on every attempt. Runtime
	// row failures from an earlier worker attempt must remain retryable.
	errs, _ := s.Validate(job.SchoolID, job.InitiatedBy, job.Kind, rows)
	validRows := map[int]bool{}
	for _, item := range errs {
		validRows[item.Row] = true
	}

	createdCount, skippedCount := job.Created, job.Skipped
	failedCount := 0
	credentialsSent, credentialFailures := job.CredentialsSent, 0
	for index, row := range rows {
		rowNumber := index + 2
		if validRows[rowNumber] {
			continue
		}
		created, skipped, _, _, _, rowErr := s.processRow(job, row)
		if skipped && job.Attempts <= 1 {
			skippedCount++
		}
		if rowErr != nil {
			failedCount++
			errs = append(errs, BulkError{Row: rowNumber, Message: rowErr.Error()})
		} else if created {
			createdCount++
		}

		values := map[string]any{
			"created": createdCount, "skipped": skippedCount, "failed": failedCount,
			"credentials_sent": credentialsSent, "credential_emails_failed": credentialFailures,
			"errors_json": mustJSON(errs), "claimed_at": time.Now().UTC(),
		}
		if err := s.updateClaimedJob(job, workerID, values); err != nil {
			// The row transaction has committed. A later queue reclaim will
			// observe the durable outbox event and resume delivery.
			return
		}
	}

	if job.SendCredentials {
		if s.db.Dialector.Name() != "postgres" {
			fail(errors.New("durable account-setup delivery requires PostgreSQL"))
			return
		}
		counts, deliveryErr := s.deliverAccountSetupNotifications(job)
		if deliveryErr != nil {
			// Fail closed but preserve the running lease. The queue can reclaim
			// the job after lease expiry and retry the durable notification.
			_ = s.updateClaimedJob(job, workerID, map[string]any{
				"last_error": "account-setup notification processing will be retried",
				"claimed_at": time.Now().UTC(),
			})
			return
		}
		credentialsSent = counts.DeliveredCount
		credentialFailures = counts.FailedCount
		if counts.PendingCount > 0 {
			_ = s.updateClaimedJob(job, workerID, map[string]any{
				"created": createdCount, "skipped": skippedCount, "failed": failedCount,
				"credentials_sent": credentialsSent, "credential_emails_failed": credentialFailures,
				"errors_json": mustJSON(errs),
				"last_error": "account-setup notifications pending retry",
				"claimed_at": time.Now().UTC(),
			})
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
