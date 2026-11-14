package service

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// openBulkWorkerPostgres creates a private schema per test so migration and
// queue state cannot collide with other packages running against TEST_DATABASE_URL.
func openBulkWorkerPostgres(t *testing.T) (*gorm.DB, string, string, string, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect PostgreSQL admin database: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	schema := "bulk_worker_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	ownerRole := "biq_owner_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	apiRole := "biq_api_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	otherRole := "biq_other_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		_ = adminSQL.Close()
		t.Fatalf("create isolated test schema: %v", err)
	}
	var existingClaimFunction bool
	if err := admin.Raw("SELECT to_regprocedure('public.claim_next_bulk_import_job(uuid)') IS NOT NULL").Scan(&existingClaimFunction).Error; err != nil || existingClaimFunction {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		_ = adminSQL.Close()
		t.Skip("TEST_DATABASE_URL already contains public.claim_next_bulk_import_job(uuid); use a clean isolated test database")
	}
	var canCreateRoles bool
	if err := admin.Raw("SELECT rolsuper OR rolcreaterole FROM pg_roles WHERE rolname = current_user").Scan(&canCreateRoles).Error; err != nil || !canCreateRoles {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		_ = adminSQL.Close()
		t.Skip("TEST_DATABASE_URL role cannot create isolated PostgreSQL roles; restricted queue tests require CREATEROLE")
	}
	if err := admin.Exec("CREATE ROLE " + ownerRole + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT BYPASSRLS").Error; err != nil {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		_ = adminSQL.Close()
		t.Skipf("cannot create isolated queue-owner test role: %v", err)
	}
	if err := admin.Exec("CREATE ROLE " + apiRole + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS").Error; err != nil {
		_ = admin.Exec("DROP ROLE " + ownerRole).Error
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		_ = adminSQL.Close()
		t.Skipf("cannot create isolated API test role: %v", err)
	}
	if err := admin.Exec("CREATE ROLE " + otherRole + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS").Error; err != nil {
		_ = admin.Exec("DROP ROLE " + apiRole).Error
		_ = admin.Exec("DROP ROLE " + ownerRole).Error
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		_ = adminSQL.Close()
		t.Skipf("cannot create isolated unrelated test role: %v", err)
	}
	// CREATEROLE test users are not necessarily superusers. Temporary
	// memberships allow SET LOCAL ROLE and are removed during cleanup.
	if err := admin.Exec("GRANT " + apiRole + ", " + ownerRole + ", " + otherRole + " TO CURRENT_USER").Error; err != nil {
		_ = admin.Exec("DROP ROLE " + otherRole).Error
		_ = admin.Exec("DROP ROLE " + apiRole).Error
		_ = admin.Exec("DROP ROLE " + ownerRole).Error
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		_ = adminSQL.Close()
		t.Skipf("cannot grant isolated test roles to test session: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SET LOCAL ROLE " + ownerRole).Error; err != nil { return err }
			return tx.Exec("DROP FUNCTION IF EXISTS public.claim_next_bulk_import_job(uuid)").Error
		})
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		_ = admin.Exec("DROP OWNED BY " + otherRole).Error
		_ = admin.Exec("DROP OWNED BY " + apiRole).Error
		_ = admin.Exec("DROP OWNED BY " + ownerRole).Error
		_ = admin.Exec("REVOKE " + apiRole + ", " + ownerRole + ", " + otherRole + " FROM CURRENT_USER").Error
		_ = admin.Exec("DROP ROLE " + otherRole).Error
		_ = admin.Exec("DROP ROLE " + apiRole).Error
		_ = admin.Exec("DROP ROLE " + ownerRole).Error
		_ = adminSQL.Close()
	})

	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect isolated PostgreSQL schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate isolated PostgreSQL schema: %v", err)
	}
	if err := installBulkClaimFunctionForTest(t, admin, schema, ownerRole, apiRole, otherRole); err != nil {
		t.Fatalf("install isolated restricted claim function: %v", err)
	}
	if err := admin.Exec("GRANT USAGE ON SCHEMA " + schema + " TO " + otherRole).Error; err != nil {
		t.Fatalf("grant schema usage to unrelated test role: %v", err)
	}
	return db, schema, ownerRole, apiRole, otherRole
}

func installBulkClaimFunctionForTest(t *testing.T, admin *gorm.DB, schema, ownerRole, apiRole, otherRole string) error {
	t.Helper()
	if err := admin.Exec("GRANT USAGE ON SCHEMA " + schema + " TO " + ownerRole + ", " + apiRole).Error; err != nil { return err }
	if err := admin.Exec("GRANT SELECT ON TABLE " + schema + ".bulk_import_jobs TO " + ownerRole).Error; err != nil { return err }
	if err := admin.Exec("GRANT UPDATE (status, worker_id, claimed_at, attempts, last_error) ON TABLE " + schema + ".bulk_import_jobs TO " + ownerRole).Error; err != nil { return err }
	if err := admin.Exec("GRANT SELECT, INSERT, UPDATE ON TABLE " + schema + ".bulk_import_jobs TO " + apiRole).Error; err != nil { return err }
	if err := admin.Exec("ALTER TABLE " + schema + ".bulk_import_jobs ENABLE ROW LEVEL SECURITY").Error; err != nil { return err }
	policies := []string{
		"CREATE POLICY bulk_import_jobs_tenant_select ON " + schema + ".bulk_import_jobs FOR SELECT TO " + apiRole + " USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid)",
		"CREATE POLICY bulk_import_jobs_tenant_insert ON " + schema + ".bulk_import_jobs FOR INSERT TO " + apiRole + " WITH CHECK (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid AND initiated_by = nullif(current_setting('app.current_user_id', true), '')::uuid)",
		"CREATE POLICY bulk_import_jobs_tenant_update ON " + schema + ".bulk_import_jobs FOR UPDATE TO " + apiRole + " USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid AND initiated_by = nullif(current_setting('app.current_user_id', true), '')::uuid) WITH CHECK (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid AND initiated_by = nullif(current_setting('app.current_user_id', true), '')::uuid)",
	}
	for _, policy := range policies {
		if err := admin.Exec(policy).Error; err != nil { return err }
	}
	if err := admin.Exec("GRANT USAGE, CREATE ON SCHEMA public TO " + ownerRole).Error; err != nil { return err }
	if err := admin.Exec("GRANT USAGE ON SCHEMA public TO " + apiRole + ", " + otherRole).Error; err != nil { return err }
	functionSQL := fmt.Sprintf(`
CREATE OR REPLACE FUNCTION public.claim_next_bulk_import_job(p_worker_id uuid)
RETURNS SETOF %s.bulk_import_jobs
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
DECLARE v_job %s.bulk_import_jobs%%ROWTYPE;
BEGIN
    IF p_worker_id IS NULL THEN RAISE EXCEPTION 'worker ID is required'; END IF;
    SELECT * INTO v_job FROM %s.bulk_import_jobs
     WHERE status = 'queued'
        OR (status = 'running' AND (claimed_at IS NULL OR claimed_at < clock_timestamp() - interval '5 minutes'))
     ORDER BY created_at ASC, id ASC LIMIT 1 FOR UPDATE SKIP LOCKED;
    IF NOT FOUND THEN RETURN; END IF;
    UPDATE %s.bulk_import_jobs
       SET status = 'running', worker_id = p_worker_id, claimed_at = clock_timestamp(),
           attempts = attempts + 1, last_error = ''
     WHERE id = v_job.id RETURNING * INTO v_job;
    RETURN NEXT v_job;
END
$function$`, schema, schema, schema, schema)
	if err := admin.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + ownerRole).Error; err != nil { return err }
		if err := tx.Exec(functionSQL).Error; err != nil { return err }
		if err := tx.Exec("REVOKE ALL ON FUNCTION public.claim_next_bulk_import_job(uuid) FROM PUBLIC").Error; err != nil { return err }
		return tx.Exec("GRANT EXECUTE ON FUNCTION public.claim_next_bulk_import_job(uuid) TO " + apiRole).Error
	}); err != nil { return err }
	if err := admin.Exec("REVOKE CREATE ON SCHEMA public FROM " + ownerRole).Error; err != nil { return err }
	return nil
}


func TestBulkWorkerPostgresClaimFunctionRestrictsDirectQueueAccess(t *testing.T) {
	db, schema, ownerRole, apiRole, otherRole := openBulkWorkerPostgres(t)
	schoolA, schoolB := uuid.New(), uuid.New()
	actorA, actorB := uuid.New(), uuid.New()
	jobA := models.BulkImportJob{
		SchoolID: schoolA, InitiatedBy: actorA, Kind: "students",
		FileName: "school-a.csv", Status: models.BulkImportQueued,
		CreatedAt: time.Now().UTC(),
	}
	jobB := models.BulkImportJob{
		SchoolID: schoolB, InitiatedBy: actorB, Kind: "students",
		FileName: "school-b.csv", Status: models.BulkImportQueued,
		CreatedAt: time.Now().UTC().Add(time.Second),
	}
	if err := db.Create(&jobA).Error; err != nil { t.Fatalf("create school A job: %v", err) }
	if err := db.Create(&jobB).Error; err != nil { t.Fatalf("create school B job: %v", err) }

	// The API role has normal table grants, but RLS limits it to the
	// transaction-local school and initiating user.
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + apiRole).Error; err != nil { return err }
		if err := tx.Exec("SELECT set_config('app.current_school_id', ?, true)", schoolA.String()).Error; err != nil { return err }
		if err := tx.Exec("SELECT set_config('app.current_user_id', ?, true)", actorA.String()).Error; err != nil { return err }

		var visible []models.BulkImportJob
		if err := tx.Table(schema + ".bulk_import_jobs").Find(&visible).Error; err != nil { return err }
		if len(visible) != 1 || visible[0].ID != jobA.ID {
			return fmt.Errorf("school A should see only its own import job, got %+v", visible)
		}

		crossUpdate := tx.Table(schema + ".bulk_import_jobs").Where("id = ?", jobB.ID).Update("last_error", "cross-tenant")
		if crossUpdate.Error != nil { return crossUpdate.Error }
		if crossUpdate.RowsAffected != 0 { return fmt.Errorf("school A updated school B job") }

		sameUpdate := tx.Table(schema + ".bulk_import_jobs").Where("id = ?", jobA.ID).Update("last_error", "same-tenant")
		if sameUpdate.Error != nil { return sameUpdate.Error }
		if sameUpdate.RowsAffected != 1 { return fmt.Errorf("school A could not update its own job") }

		crossInsert := models.BulkImportJob{SchoolID: schoolB, InitiatedBy: actorB, Kind: "students", FileName: "cross.csv", Status: models.BulkImportPending}
		if err := tx.SavePoint("before_cross_tenant_insert").Error; err != nil { return err }
		if err := tx.Create(&crossInsert).Error; err == nil { return fmt.Errorf("cross-tenant job insert was not rejected") }
		if err := tx.RollbackTo("before_cross_tenant_insert").Error; err != nil { return err }

		sameInsert := models.BulkImportJob{SchoolID: schoolA, InitiatedBy: actorA, Kind: "students", FileName: "same.csv", Status: models.BulkImportPending}
		if err := tx.Create(&sameInsert).Error; err != nil { return fmt.Errorf("same-tenant job insert failed: %w", err) }
		return nil
	})
	if err != nil { t.Fatalf("tenant-scoped API table access failed: %v", err) }

	// Without transaction-local context, SELECT sees no jobs and INSERT fails.
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + apiRole).Error; err != nil { return err }
		var count int64
		if err := tx.Table(schema + ".bulk_import_jobs").Count(&count).Error; err != nil { return err }
		if count != 0 { return fmt.Errorf("expected no-context SELECT to see zero jobs, got %d", count) }
		return tx.Create(&models.BulkImportJob{SchoolID: schoolA, InitiatedBy: actorA, Kind: "students", FileName: "no-context.csv", Status: models.BulkImportPending}).Error
	})
	if err == nil { t.Fatal("API role unexpectedly inserted a job without tenant context") }

	// Global claim is the only cross-tenant queue operation and is available
	// through the SECURITY DEFINER function even without tenant GUCs.
	workerID := uuid.New()
	var claimed models.BulkImportJob
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + apiRole).Error; err != nil { return err }
		result := tx.Raw("SELECT * FROM public.claim_next_bulk_import_job(?)", workerID).Scan(&claimed)
		if result.Error != nil { return result.Error }
		if result.RowsAffected != 1 { return fmt.Errorf("expected one claimed job, got %d", result.RowsAffected) }
		return nil
	})
	if err != nil { t.Fatalf("restricted role could not execute queue claim function: %v", err) }
	if claimed.ID != jobA.ID || claimed.WorkerID == nil || *claimed.WorkerID != workerID {
		t.Fatalf("claim function returned unexpected job/worker: %+v", claimed)
	}

	for _, statement := range []string{
		"DELETE FROM " + schema + ".bulk_import_jobs",
		"TRUNCATE TABLE " + schema + ".bulk_import_jobs",
	} {
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec("SET LOCAL ROLE " + apiRole).Error; err != nil { return err }
			return tx.Exec(statement).Error
		})
		if err == nil { t.Fatalf("API role unexpectedly executed forbidden queue statement: %s", statement) }
	}

	var apiCanAssumeQueueOwner bool
	if err := db.Raw("SELECT pg_has_role(?, ?, 'MEMBER')", apiRole, ownerRole).Scan(&apiCanAssumeQueueOwner).Error; err != nil {
		t.Fatalf("inspect API-to-owner role membership: %v", err)
	}
	if apiCanAssumeQueueOwner { t.Fatal("API role unexpectedly belongs to the queue-owner role") }

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + otherRole).Error; err != nil { return err }
		var denied models.BulkImportJob
		return tx.Raw("SELECT * FROM public.claim_next_bulk_import_job(?)", uuid.New()).Scan(&denied).Error
	})
	if err == nil { t.Fatal("unrelated role unexpectedly executed the queue claim function") }

	// The function owner can read the queue and update lease columns only.
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + ownerRole).Error; err != nil { return err }
		var count int64
		if err := tx.Table(schema + ".bulk_import_jobs").Count(&count).Error; err != nil { return err }
		return tx.Exec("UPDATE " + schema + ".bulk_import_jobs SET rows_json = '{}'").Error
	})
	if err == nil { t.Fatal("queue owner unexpectedly updated protected payload columns") }
}

func TestBulkWorkerPostgresClaimFunctionReclaimsOnlyExpiredLeases(t *testing.T) {
	db, _, _, _, _ := openBulkWorkerPostgres(t)
	schoolID, actor := uuid.New(), uuid.New()
	oldWorker, legacyWorker, activeWorker := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	jobs := []models.BulkImportJob{
		{SchoolID: schoolID, InitiatedBy: actor, Kind: "students", FileName: "expired.csv",
			Status: models.BulkImportRunning, WorkerID: &oldWorker, ClaimedAt: ptrTime(now.Add(-6 * time.Minute)),
			Attempts: 1, CreatedAt: now.Add(-3 * time.Minute)},
		{SchoolID: schoolID, InitiatedBy: actor, Kind: "students", FileName: "legacy.csv",
			Status: models.BulkImportRunning, WorkerID: &legacyWorker, ClaimedAt: nil,
			Attempts: 2, CreatedAt: now.Add(-2 * time.Minute)},
		{SchoolID: schoolID, InitiatedBy: actor, Kind: "students", FileName: "active.csv",
			Status: models.BulkImportRunning, WorkerID: &activeWorker, ClaimedAt: ptrTime(now),
			Attempts: 3, CreatedAt: now.Add(-time.Minute)},
	}
	for i := range jobs {
		if err := db.Create(&jobs[i]).Error; err != nil { t.Fatalf("create lease fixture %d: %v", i, err) }
	}
	worker := NewBulkImportService(db, nil)
	first, err := worker.claimNextJob()
	if err != nil { t.Fatalf("claim expired lease: %v", err) }
	if first == nil || first.ID != jobs[0].ID || first.Attempts != 2 {
		t.Fatalf("expected expired lease to be reclaimed and attempts incremented: %+v", first)
	}
	second, err := worker.claimNextJob()
	if err != nil { t.Fatalf("claim legacy lease: %v", err) }
	if second == nil || second.ID != jobs[1].ID || second.Attempts != 3 {
		t.Fatalf("expected legacy unleased job to be reclaimed: %+v", second)
	}
	third, err := worker.claimNextJob()
	if err != nil { t.Fatalf("check active lease: %v", err) }
	if third != nil { t.Fatalf("active unexpired lease must not be reclaimed: %+v", third) }
}

func ptrTime(value time.Time) *time.Time { return &value }


func TestBulkWorkerPostgresConcurrentClaimsSkipLockedJobs(t *testing.T) {
	db, _, _, _, _ := openBulkWorkerPostgres(t)
	schoolID, actor := uuid.New(), uuid.New()
	first := models.BulkImportJob{
		SchoolID: schoolID, InitiatedBy: actor, Kind: "students",
		FileName: "first.csv", Status: models.BulkImportQueued,
		CreatedAt: time.Now().UTC().Add(-time.Minute),
	}
	second := models.BulkImportJob{
		SchoolID: schoolID, InitiatedBy: actor, Kind: "students",
		FileName: "second.csv", Status: models.BulkImportQueued,
		CreatedAt: time.Now().UTC(),
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first queued job: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second queued job: %v", err)
	}

	// Hold the oldest job's row lock as if another worker is claiming it.
	// A concurrent claimant must skip that row and claim the second job.
	lockTx := db.Begin()
	if lockTx.Error != nil {
		t.Fatalf("begin lock transaction: %v", lockTx.Error)
	}
	var locked models.BulkImportJob
	if err := lockTx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", first.ID).First(&locked).Error; err != nil {
		_ = lockTx.Rollback().Error
		t.Fatalf("lock oldest queued job: %v", err)
	}

	workerA := NewBulkImportService(db, nil)
	workerB := NewBulkImportService(db, nil)
	type claimResult struct {
		job *models.BulkImportJob
		err error
	}
	resultCh := make(chan claimResult, 1)
	go func() {
		job, err := workerA.claimNextJob()
		resultCh <- claimResult{job: job, err: err}
	}()

	var firstClaim claimResult
	select {
	case firstClaim = <-resultCh:
	case <-time.After(5 * time.Second):
		_ = lockTx.Rollback().Error
		t.Fatal("claim blocked on the locked row; PostgreSQL SKIP LOCKED was not applied")
	}
	if firstClaim.err != nil {
		_ = lockTx.Rollback().Error
		t.Fatalf("first concurrent claim: %v", firstClaim.err)
	}
	if firstClaim.job == nil || firstClaim.job.ID != second.ID {
		_ = lockTx.Rollback().Error
		t.Fatalf("expected worker A to skip locked oldest job and claim %s, got %+v", second.ID, firstClaim.job)
	}

	if err := lockTx.Commit().Error; err != nil {
		t.Fatalf("release oldest job lock: %v", err)
	}
	secondClaim, err := workerB.claimNextJob()
	if err != nil {
		t.Fatalf("second worker claim after lock release: %v", err)
	}
	if secondClaim == nil || secondClaim.ID != first.ID {
		t.Fatalf("expected worker B to claim released oldest job %s, got %+v", first.ID, secondClaim)
	}
	if firstClaim.job.ID == secondClaim.ID {
		t.Fatal("two workers claimed the same job")
	}
	if firstClaim.job.WorkerID == nil || *firstClaim.job.WorkerID != workerA.workerID {
		t.Fatal("first job does not record worker A ownership")
	}
	if secondClaim.WorkerID == nil || *secondClaim.WorkerID != workerB.workerID {
		t.Fatal("second job does not record worker B ownership")
	}

	var claimed int64
	if err := db.Model(&models.BulkImportJob{}).
		Where("status = ? AND worker_id IN ?", models.BulkImportRunning, []uuid.UUID{workerA.workerID, workerB.workerID}).
		Count(&claimed).Error; err != nil {
		t.Fatalf("count claimed jobs: %v", err)
	}
	if claimed != 2 {
		t.Fatalf("expected two distinct running claims, got %d (%s)", claimed, fmt.Sprint(claimed))
	}
}
