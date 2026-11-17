package service

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/security/bulkimportcontext"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestBulkImportCredentialOutboxWorkerCrashRetry(t *testing.T) {
	adminURL := os.Getenv("BULK_CONTEXT_TEST_DATABASE_URL")
	workerURL := os.Getenv("BULK_CONTEXT_TEST_WORKER_URL")
	keyHex := os.Getenv("BULK_CONTEXT_TEST_SIGNING_KEY_HEX")
	if adminURL == "" || workerURL == "" || keyHex == "" {
		t.Skip("isolated signed-context PostgreSQL fixture is not configured")
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil { t.Fatalf("decode CI verifier key: %v", err) }
	signer, err := bulkimportcontext.NewSigner("ci-outbox-key", key, 30*time.Second)
	if err != nil { t.Fatalf("create CI assertion signer: %v", err) }

	admin, err := gorm.Open(postgres.Open(adminURL), &gorm.Config{})
	if err != nil { t.Fatalf("connect fixture owner database: %v", err) }
	adminSQL, err := admin.DB()
	if err != nil { t.Fatal(err) }
	defer adminSQL.Close()

	workerDB, err := gorm.Open(postgres.Open(workerURL), &gorm.Config{})
	if err != nil { t.Fatalf("connect restricted processing login: %v", err) }
	workerSQL, err := workerDB.DB()
	if err != nil { t.Fatal(err) }
	workerSQL.SetMaxOpenConns(2)
	defer workerSQL.Close()

	schoolID, initiatorID := uuid.New(), uuid.New()
	jobID, workerID, leaseID := uuid.New(), uuid.New(), uuid.New()
	email := "outbox-recovery-" + uuid.NewString() + "@example.test"
	if err := admin.Exec(
		"INSERT INTO public.users(id,school_id,name,email,password_hash,role,active) VALUES (?,?,?,?,?,?,true)",
		initiatorID, schoolID, "Fixture Administrator", "admin-"+uuid.NewString()+"@example.test", "test-only-hash", "school_admin",
	).Error; err != nil { t.Fatalf("create initiating user: %v", err) }

	job := models.BulkImportJob{
		ID: jobID, SchoolID: schoolID, InitiatedBy: initiatorID,
		Kind: "students", FileName: "recovery.csv", Status: models.BulkImportRunning,
		SendCredentials: true, RowsJSON: "", ErrorsJSON: "[]",
		WorkerID: &workerID, LeaseID: &leaseID, ClaimedAt: timePtr(time.Now().UTC()),
		Attempts: 1,
	}
	insertJob := "INSERT INTO public.bulk_import_jobs " +
		"(id,school_id,initiated_by,kind,file_name,status,send_credentials,rows_json,errors_json,worker_id,lease_id,claimed_at,attempts,created_at,updated_at) " +
		"VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,clock_timestamp(),clock_timestamp())"
	rows := []BulkRow{{"name":"Imported Student","email":email,"admission_number":"OUTBOX-001"}}
	rowsJSON, err := json.Marshal(rows)
	if err != nil { t.Fatal(err) }
	job.RowsJSON = string(rowsJSON)
	if err := admin.Exec(insertJob,
		job.ID, job.SchoolID, job.InitiatedBy, job.Kind, job.FileName, job.Status,
		job.SendCredentials, job.RowsJSON, job.ErrorsJSON, workerID, leaseID, job.ClaimedAt, job.Attempts,
	).Error; err != nil { t.Fatalf("create active import job: %v", err) }

	mailerCalls := atomic.Int32{}
	var receivedBody string
	var receivedIdempotencyKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mailerCalls.Add(1)
		receivedIdempotencyKey = r.Header.Get("Idempotency-Key")
		var payload struct { Text string `json:"text"` }
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil { t.Errorf("decode notice request: %v", err) }
		receivedBody = payload.Text
		if mailerCalls.Load() == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("transient test outage"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"test-delivery-accepted"}`))
	}))
	defer server.Close()

	mailer := &CredentialMailer{
		host: "smtp.example.test", from: "noreply@stonezdigital.com.ng",
		loginURL: "https://app.stonezdigital.com.ng/login",
		resendAPIKey: "re_test_key", resendAPIURL: server.URL,
		httpClient: server.Client(),
	}
	worker := NewBulkImportService(workerDB, mailer)
	if err := worker.RequireSignedContext(signer); err != nil { t.Fatalf("enable signed worker context: %v", err) }

	// The account, student profile and outbox event commit in one signed
	// transaction. No plaintext temporary password leaves the row handler.
	created, skipped, _, temporaryPassword, _, err := worker.processStudent(job, "Imported Student", email, rows[0])
	if err != nil { t.Fatalf("create imported account and durable outbox event: %v", err) }
	if !created || skipped || temporaryPassword != "" {
		t.Fatalf("row handler exposed an unexpected credential or creation state: created=%t skipped=%t password=%q", created, skipped, temporaryPassword)
	}
	var accountID uuid.UUID
	if err := admin.Raw("SELECT id FROM public.users WHERE email=?", email).Scan(&accountID).Error; err != nil { t.Fatal(err) }
	if accountID == uuid.Nil { t.Fatal("account transaction did not persist the imported user") }
	var eventID string
	if err := admin.Raw("SELECT event_id::text FROM stonez_context_private.account_setup_outbox WHERE job_id=? AND user_id=?", jobID, accountID).Scan(&eventID).Error; err != nil { t.Fatal(err) }
	var createdProgress int
	if err := admin.Raw("SELECT created FROM public.bulk_import_jobs WHERE id=?", jobID).Scan(&createdProgress).Error; err != nil { t.Fatal(err) }
	if createdProgress != 1 { t.Fatalf("created-row progress was not committed atomically with the account and outbox: %d", createdProgress) }

	// The original worker exits after the account transaction. A new worker
	// receives a fresh job lease before attempting delivery.
	firstRecoveryWorkerID, firstRecoveryLeaseID := uuid.New(), uuid.New()
	if err := admin.Exec(
		"UPDATE public.bulk_import_jobs SET worker_id=?,lease_id=?,claimed_at=clock_timestamp(),attempts=attempts+1 WHERE id=? AND status='running'",
		firstRecoveryWorkerID, firstRecoveryLeaseID, jobID,
	).Error; err != nil { t.Fatal(err) }
	job.WorkerID, job.LeaseID, job.ClaimedAt = &firstRecoveryWorkerID, &firstRecoveryLeaseID, timePtr(time.Now().UTC())
	job.Attempts = 2
	worker.processClaimed(job, firstRecoveryWorkerID)
	if mailerCalls.Load() != 1 { t.Fatalf("expected first delivery attempt, got %d", mailerCalls.Load()) }

	var outboxState struct { Status string; AttemptCount int }
	if err := admin.Raw(
		"SELECT status,attempt_count FROM stonez_context_private.account_setup_outbox WHERE job_id=? AND user_id=?",
		jobID, accountID,
	).Scan(&outboxState).Error; err != nil { t.Fatal(err) }
	if outboxState.Status != "pending" || outboxState.AttemptCount != 1 {
		t.Fatalf("provider failure was not durably scheduled for retry: %+v", outboxState)
	}
	if err := admin.First(&job, "id = ?", jobID).Error; err != nil { t.Fatalf("reload persisted job progress: %v", err) }

	// Reclaim the still-running job with a fresh worker/lease, as happens after
	// the previous worker process exits or crashes.
	newWorkerID, newLeaseID := uuid.New(), uuid.New()
	if err := admin.Exec(
		"UPDATE stonez_context_private.account_setup_outbox SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE job_id=? AND user_id=? AND status='pending'",
		jobID, accountID,
	).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec(
		"UPDATE public.bulk_import_jobs SET worker_id=?,lease_id=?,claimed_at=clock_timestamp(),attempts=attempts+1 WHERE id=? AND status='running'",
		newWorkerID, newLeaseID, jobID,
	).Error; err != nil { t.Fatal(err) }
	job.WorkerID, job.LeaseID, job.ClaimedAt = &newWorkerID, &newLeaseID, timePtr(time.Now().UTC())
	job.Attempts = 3

	restartedWorker := NewBulkImportService(workerDB, mailer)
	if err := restartedWorker.RequireSignedContext(signer); err != nil { t.Fatal(err) }
	restartedWorker.processClaimed(job, newWorkerID)

	if mailerCalls.Load() != 2 { t.Fatalf("expected retry after worker restart, got %d calls", mailerCalls.Load()) }
	if receivedIdempotencyKey != "stonez-account-setup-"+eventID {
		t.Fatalf("provider idempotency key is not tied to the durable event: %q", receivedIdempotencyKey)
	}
	for _, forbidden := range []string{"temporary password:", "one-time-token", "password_hash"} {
		if strings.Contains(strings.ToLower(receivedBody), strings.ToLower(forbidden)) {
			t.Fatalf("account-setup email contains forbidden credential material %q", forbidden)
		}
	}
	if !strings.Contains(strings.ToLower(receivedBody), "forgot password") {
		t.Fatal("notice does not direct the recipient to the password-reset flow")
	}

	var final struct { Status string; AttemptCount int }
	if err := admin.Raw(
		"SELECT status,attempt_count FROM stonez_context_private.account_setup_outbox WHERE job_id=? AND user_id=?",
		jobID, accountID,
	).Scan(&final).Error; err != nil { t.Fatal(err) }
	if final.Status != "delivered" || final.AttemptCount != 2 {
		t.Fatalf("restarted worker did not complete the durable event: %+v", final)
	}
	var finalJob struct { Status string; CredentialsSent int; CredentialEmailsFailed int }
	if err := admin.Raw("SELECT status,credentials_sent,credential_emails_failed FROM public.bulk_import_jobs WHERE id=?", jobID).Scan(&finalJob).Error; err != nil { t.Fatal(err) }
	if finalJob.Status != models.BulkImportCompleted || finalJob.CredentialsSent != 1 || finalJob.CredentialEmailsFailed != 0 {
		t.Fatalf("job summary does not reflect durable delivery: %+v", finalJob)
	}
}

func TestAccountSetupRetryDelayIsBounded(t *testing.T) {
	cases := []struct { attempt int; want time.Duration }{
		{0, 30*time.Second}, {1, 30*time.Second}, {2, 60*time.Second},
		{3, 120*time.Second}, {7, 1920*time.Second}, {8, 3600*time.Second}, {20, 3600*time.Second},
	}
	for _, tc := range cases {
		if got := accountSetupRetryDelay(tc.attempt); got != tc.want {
			t.Errorf("attempt %d: got %s, want %s", tc.attempt, got, tc.want)
		}
	}
}
