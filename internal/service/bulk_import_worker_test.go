package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
)

func TestBulkWorkerClaimsQueuedJobsExactlyOnce(t *testing.T) {
	db := bulkTestDB(t)
	schoolID, actor := uuid.New(), uuid.New()
	jobs := []models.BulkImportJob{
		{SchoolID: schoolID, InitiatedBy: actor, Kind: "students", FileName: "a.csv", Status: models.BulkImportQueued},
		{SchoolID: schoolID, InitiatedBy: actor, Kind: "students", FileName: "b.csv", Status: models.BulkImportQueued},
	}
	for i := range jobs {
		if err := db.Create(&jobs[i]).Error; err != nil { t.Fatal(err) }
	}
	worker := NewBulkImportService(db, nil)
	first, err := worker.claimNextJob()
	if err != nil || first == nil { t.Fatalf("first claim: job=%v err=%v", first, err) }
	second, err := worker.claimNextJob()
	if err != nil || second == nil { t.Fatalf("second claim: job=%v err=%v", second, err) }
	if first.ID == second.ID { t.Fatal("same queued job was claimed twice") }
	third, err := worker.claimNextJob()
	if err != nil { t.Fatal(err) }
	if third != nil { t.Fatalf("expected no unclaimed job, got %s", third.ID) }
	if first.WorkerID == nil || *first.WorkerID != worker.workerID { t.Fatal("claim did not persist worker ownership") }
}

func TestBulkWorkerReclaimsExpiredLease(t *testing.T) {
	db := bulkTestDB(t)
	schoolID, actor, oldWorker := uuid.New(), uuid.New(), uuid.New()
	expired := time.Now().UTC().Add(-2 * bulkImportClaimLease)
	job := models.BulkImportJob{
		SchoolID: schoolID, InitiatedBy: actor, Kind: "students", FileName: "retry.csv",
		Status: models.BulkImportRunning, WorkerID: &oldWorker, ClaimedAt: &expired, Attempts: 1,
	}
	if err := db.Create(&job).Error; err != nil { t.Fatal(err) }
	worker := NewBulkImportService(db, nil)
	claimed, err := worker.claimNextJob()
	if err != nil || claimed == nil { t.Fatalf("expected expired job to be reclaimed: job=%v err=%v", claimed, err) }
	if claimed.ID != job.ID { t.Fatalf("claimed wrong job: %s", claimed.ID) }
	if claimed.Attempts != 2 { t.Fatalf("expected attempt 2, got %d", claimed.Attempts) }
	if claimed.WorkerID == nil || *claimed.WorkerID != worker.workerID { t.Fatal("expired lease was not reassigned") }
}

func TestBulkWorkerRejectsInitiatorFromDifferentSchool(t *testing.T) {
	db := bulkTestDB(t)
	schoolID, otherSchool, actor, workerID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	user := models.User{ID: actor, Name: "Wrong School", Email: "wrong-school@example.com", Role: "school_admin", Active: true, SchoolID: &otherSchool}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	job := models.BulkImportJob{
		ID: uuid.New(), SchoolID: schoolID, InitiatedBy: actor, Kind: "students", FileName: "bad.csv",
		Status: models.BulkImportRunning, WorkerID: &workerID, ClaimedAt: timePtr(time.Now().UTC()),
	}
	if err := db.Create(&job).Error; err != nil { t.Fatal(err) }
	worker := NewBulkImportService(db, nil)
	worker.processClaimed(job, workerID)
	var got models.BulkImportJob
	if err := db.First(&got, job.ID).Error; err != nil { t.Fatal(err) }
	if got.Status != models.BulkImportFailed { t.Fatalf("expected failed status for cross-school initiator, got %q", got.Status) }
}

func timePtr(v time.Time) *time.Time { return &v }
