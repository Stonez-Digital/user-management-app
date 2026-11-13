package service

import (
	"encoding/json"
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

func TestBulkWorkerRecoveryDoesNotDuplicatePreviouslyImportedStudent(t *testing.T) {
	db := bulkTestDB(t)
	schoolID, actor := uuid.New(), uuid.New()
	actorUser := models.User{ID: actor, Name: "School Admin", Email: "admin-recovery@example.com", Role: "school_admin", Active: true, SchoolID: &schoolID}
	if err := db.Create(&actorUser).Error; err != nil { t.Fatal(err) }
	actor = actorUser.ID
	studentUser := models.User{ID: uuid.New(), Name: "Recovered Student", Email: "recovered@example.com", Role: "student", Active: true, SchoolID: &schoolID}
	if err := db.Create(&studentUser).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&models.Student{SchoolID: schoolID, UserID: studentUser.ID, AdmissionNumber: "REC-001"}).Error; err != nil { t.Fatal(err) }
	rows, _ := json.Marshal([]BulkRow{{"name":"Recovered Student","email":"recovered@example.com","admission_number":"REC-001"}})
	expired := time.Now().UTC().Add(-2 * bulkImportClaimLease)
	job := models.BulkImportJob{SchoolID:schoolID,InitiatedBy:actor,Kind:"students",FileName:"recovery.csv",Status:models.BulkImportRunning,RowsJSON:string(rows),ClaimedAt:&expired,Attempts:1}
	if err := db.Create(&job).Error; err != nil { t.Fatal(err) }
	worker := NewBulkImportService(db,nil)
	claimed, err := worker.claimNextJob()
	if err != nil || claimed == nil { t.Fatalf("claim recovery job: job=%v err=%v",claimed,err) }
	worker.processClaimed(*claimed,worker.workerID)
	var got models.BulkImportJob
	if err := db.First(&got,job.ID).Error; err != nil { t.Fatal(err) }
	if got.Status != models.BulkImportCompleted || got.Skipped != 1 || got.Attempts != 2 {
		t.Fatalf("unexpected recovered job: status=%s skipped=%d attempts=%d",got.Status,got.Skipped,got.Attempts)
	}
	var students int64
	if err := db.Model(&models.Student{}).Where("school_id = ? AND admission_number = ?",schoolID,"REC-001").Count(&students).Error; err != nil { t.Fatal(err) }
	if students != 1 { t.Fatalf("expected one student after recovery, got %d",students) }
}

func TestBulkImportRowRollsBackStudentAndUserWhenEnrollmentValidationFails(t *testing.T) {
	db := bulkTestDB(t)
	schoolID, actor := uuid.New(), uuid.New()
	rows, _ := json.Marshal([]BulkRow{{"name":"Rollback Student","email":"rollback@example.com","admission_number":"ROLL-001","session":"2026/2027","class":"Missing Class","section":"A"}})
	job := models.BulkImportJob{SchoolID:schoolID,InitiatedBy:actor,Kind:"students",FileName:"rollback.csv",Status:models.BulkImportPending,RowsJSON:string(rows)}
	if err := db.Create(&job).Error; err != nil { t.Fatal(err) }
	NewBulkImportService(db,nil).run(job)
	var students, users int64
	if err := db.Model(&models.Student{}).Where("school_id = ? AND admission_number = ?",schoolID,"ROLL-001").Count(&students).Error; err != nil { t.Fatal(err) }
	if err := db.Model(&models.User{}).Where("school_id = ? AND email = ?",schoolID,"rollback@example.com").Count(&users).Error; err != nil { t.Fatal(err) }
	if students != 0 || users != 0 { t.Fatalf("expected row transaction rollback, got students=%d users=%d",students,users) }
}
