package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/security/bulkcontext"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const bulkImportPollInterval = 2 * time.Second
const bulkImportClaimLease = 5 * time.Minute
const bulkImportIdleTimeout = 30 * time.Second

// withTenantTransaction establishes an authenticated, transaction-bound tenant
// context. PostgreSQL authorization never trusts caller-controlled tenant GUCs.
func (s *BulkImportService) withTenantTransaction(job models.BulkImportJob, fn func(*gorm.DB) error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if s.contextSigner == nil {
				return errors.New("bulk import PostgreSQL processing requires the signed tenant-context signer")
			}
			if job.WorkerID == nil || job.LeaseGeneration <= 0 {
				return errors.New("bulk import job is missing its claimed worker lease generation")
			}
			transactionIdentity, err := database.CurrentBulkImportTransactionIdentity(tx)
			if err != nil {
				return err
			}
			assertion, err := s.contextSigner.Sign(tx.Statement.Context, bulkcontext.Binding{
				JobID: job.ID, SchoolID: job.SchoolID, InitiatedBy: job.InitiatedBy,
				WorkerID: *job.WorkerID, BackendPID: transactionIdentity.BackendPID,
				TransactionID: transactionIdentity.TransactionID, LeaseGeneration: job.LeaseGeneration,
			}, time.Minute)
			if err != nil {
				return fmt.Errorf("sign bulk import tenant context: %w", err)
			}
			if err := database.VerifyBulkImportContext(tx, assertion); err != nil {
				return err
			}
		} else if job.WorkerID != nil {
			// SQLite is used only by local business-logic tests. This check is
			// not a substitute for PostgreSQL's signed assertion and RLS boundary.
			var lease models.BulkImportJob
			cutoff := time.Now().UTC().Add(-bulkImportClaimLease)
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND school_id = ? AND initiated_by = ? AND worker_id = ? AND status = ? AND claimed_at >= ?",
					job.ID, job.SchoolID, job.InitiatedBy, *job.WorkerID, models.BulkImportRunning, cutoff).
				First(&lease).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("bulk import worker lease was lost or expired")
				}
				return err
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
		result := tx.Model(&models.BulkImportJob{}).
			Where("id = ? AND school_id = ? AND initiated_by = ?", job.ID, job.SchoolID, job.InitiatedBy).
			Updates(map[string]any{
				"status": models.BulkImportRunning,
				"worker_id": workerID,
				"claimed_at": now,
				"attempts": gorm.Expr("attempts + 1"),
				"lease_generation": gorm.Expr("lease_generation + 1"),
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
		job.LeaseGeneration++
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
	if s.db.Dialector.Name() == "postgres" && s.contextSigner == nil {
		return errors.New("bulk import PostgreSQL worker requires a Vault-backed signed-context signer")
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
		contextSigner: s.contextSigner,
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
