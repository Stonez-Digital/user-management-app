package service

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func bulkQueueTimePtr(value time.Time) *time.Time { return &value }

type restrictedClaimRow struct {
	ID         uuid.UUID `gorm:"column:id"`
	SchoolID   uuid.UUID `gorm:"column:school_id"`
	InitiatedBy uuid.UUID `gorm:"column:initiated_by"`
	Kind       string    `gorm:"column:kind"`
	FileName   string    `gorm:"column:file_name"`
	RowsJSON   string    `gorm:"column:rows_json"`
	ErrorsJSON string    `gorm:"column:errors_json"`
	Attempts   int       `gorm:"column:attempts"`
	WorkerID   *uuid.UUID `gorm:"column:worker_id"`
	ClaimedAt  *time.Time `gorm:"column:claimed_at"`
	LastError  string     `gorm:"column:last_error"`
}

// TestBulkQueuePostgresRestrictedRoleBoundary is deliberately opt-in because
// PostgreSQL role creation is cluster-wide even when all tables are isolated.
// It proves the role/permission model with a disposable SECURITY DEFINER
// fixture; it does not test or install the production claim function.
func TestBulkQueuePostgresRestrictedRoleBoundary(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("BULK_QUEUE_ROLE_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("BULK_QUEUE_ROLE_TEST_DATABASE_URL (or TEST_DATABASE_URL) is not configured")
	}
	if os.Getenv("BULK_QUEUE_ROLE_TEST_ALLOW_ROLE_DDL") != "true" {
		t.Skip("set BULK_QUEUE_ROLE_TEST_ALLOW_ROLE_DDL=true only for a disposable PostgreSQL test database")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		t.Fatalf("TEST_DATABASE_URL must be a PostgreSQL URL for this role test")
	}
	expectedDB := strings.TrimSpace(os.Getenv("BULK_QUEUE_ROLE_TEST_DATABASE"))
	if expectedDB == "" {
		t.Fatal("BULK_QUEUE_ROLE_TEST_DATABASE must explicitly name the disposable test database")
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect role-test database: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer adminSQL.Close()

	var identity struct {
		Database string
		Role     string
		Super    bool
		CreateRole bool
	}
	if err := admin.Raw(`SELECT current_database() AS database,
		current_user AS role,
		r.rolsuper AS super,
		r.rolcreaterole AS create_role
		FROM pg_roles r WHERE r.rolname = current_user`).Scan(&identity).Error; err != nil {
		t.Fatalf("inspect test connection identity: %v", err)
	}
	if identity.Database != expectedDB {
		t.Fatalf("connected database %q does not match explicit disposable database %q", identity.Database, expectedDB)
	}
	lowerDB := strings.ToLower(identity.Database)
	if strings.Contains(lowerDB, "prod") || strings.Contains(lowerDB, "production") {
		t.Fatalf("refusing role DDL against production-looking database %q", identity.Database)
	}
	if identity.Super {
		t.Fatal("role-boundary fixture must not run as a superuser")
	}
	if !identity.CreateRole {
		t.Fatal("opted-in role test identity lacks CREATEROLE")
	}

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	schema := "bq_role_test_" + suffix
	ownerRole := "bq_owner_" + suffix
	runtimeRole := "bq_runtime_" + suffix
	runtimeLogin := "bq_login_" + suffix
	outsiderLogin := "bq_other_" + suffix
	runtimePassword := uuid.NewString() + "Aa9!"
	outsiderPassword := uuid.NewString() + "Bb8!"
	created := false
	defer func() {
		if !created {
			return
		}
		_ = admin.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
		for _, role := range []string{runtimeLogin, outsiderLogin, runtimeRole, ownerRole} {
			_ = admin.Exec("DROP ROLE IF EXISTS " + role).Error
		}
	}()

	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	created = true
	if err := admin.Exec("CREATE ROLE " + ownerRole + " NOLOGIN NOSUPERUSER NOBYPASSRLS").Error; err != nil {
		t.Fatalf("create queue-owner role: %v", err)
	}
	if err := admin.Exec("CREATE ROLE " + runtimeRole + " NOLOGIN NOSUPERUSER NOBYPASSRLS").Error; err != nil {
		t.Fatalf("create runtime group role: %v", err)
	}
	if err := admin.Exec(fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER NOBYPASSRLS", runtimeLogin, runtimePassword)).Error; err != nil {
		t.Fatalf("create restricted runtime login: %v", err)
	}
	if err := admin.Exec(fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s' NOSUPERUSER NOBYPASSRLS", outsiderLogin, outsiderPassword)).Error; err != nil {
		t.Fatalf("create unrelated login: %v", err)
	}
	if err := admin.Exec("GRANT " + runtimeRole + " TO " + runtimeLogin).Error; err != nil {
		t.Fatalf("grant runtime group membership: %v", err)
	}
	if err := admin.Exec("GRANT USAGE ON SCHEMA " + schema + " TO " + ownerRole + ", " + runtimeRole + ", " + outsiderLogin).Error; err != nil {
		t.Fatalf("grant schema usage: %v", err)
	}

	table := schema + ".bulk_import_jobs"
	statements := []string{
		`CREATE TABLE ` + table + ` (
			id uuid PRIMARY KEY, school_id uuid NOT NULL, initiated_by uuid NOT NULL,
			kind text NOT NULL, file_name text NOT NULL, status text NOT NULL,
			rows_json text NOT NULL, errors_json text NOT NULL,
			worker_id uuid, claimed_at timestamptz, attempts integer NOT NULL DEFAULT 0,
			last_error text NOT NULL DEFAULT '', created_at timestamptz NOT NULL
		)`,
		"ALTER TABLE " + table + " ENABLE ROW LEVEL SECURITY",
		"CREATE POLICY queue_owner_all ON " + table + " FOR ALL TO " + ownerRole + " USING (true) WITH CHECK (true)",
		"GRANT SELECT (id, school_id, initiated_by, kind, file_name, status, rows_json, errors_json, created_at, attempts, claimed_at, worker_id, last_error) ON " + table + " TO " + ownerRole,
		"GRANT UPDATE (status, worker_id, claimed_at, attempts, last_error) ON " + table + " TO " + ownerRole,
		"REVOKE ALL ON " + table + " FROM PUBLIC",
	}
	for _, statement := range statements {
		if err := admin.Exec(statement).Error; err != nil {
			t.Fatalf("configure isolated queue table (%s): %v", statement, err)
		}
	}

	jobID, schoolID, actorID := uuid.New(), uuid.New(), uuid.New()
	queuedID, unleasedID, activeID := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	seed := "INSERT INTO " + table + " (id, school_id, initiated_by, kind, file_name, status, rows_json, errors_json, created_at, claimed_at, attempts, last_error) VALUES (?, ?, ?, 'students', ?, ?, ?, ?, ?, ?, ?, ?)"
	seeds := []struct {
		id uuid.UUID
		name, status, rows, errors string
		created time.Time
		claimed *time.Time
		attempts int
		lastError string
	}{
		{jobID, "expired.csv", "running", `[{"name":"Expired payload"}]`, `[{"row":2,"message":"previous"}]`, now.Add(-4*time.Minute), bulkQueueTimePtr(now.Add(-10*time.Minute)), 2, "stale failure"},
		{queuedID, "queued.csv", "queued", `[{"name":"Queued payload"}]`, `[{"row":3,"message":"validation"}]`, now.Add(-3*time.Minute), nil, 0, "old queue error"},
		{unleasedID, "unleased.csv", "running", "[]", "[]", now.Add(-2*time.Minute), nil, 4, "orphaned lease"},
		{activeID, "active.csv", "running", "[]", "[]", now.Add(-time.Minute), bulkQueueTimePtr(now.Add(-time.Minute)), 1, ""},
	}
	for _, s := range seeds {
		if err := admin.Exec(seed, s.id, schoolID, actorID, s.name, s.status, s.rows, s.errors, s.created, s.claimed, s.attempts, s.lastError).Error; err != nil {
			t.Fatalf("seed isolated queue job %s: %v", s.name, err)
		}
	}

	// The fixture function is intentionally small: it validates SECURITY DEFINER
	// ownership and grants without pretending the production function is done.
	function := schema + ".claim_bulk_import_job"
	createFunction := `CREATE FUNCTION ` + function + `(p_worker uuid)
		RETURNS TABLE(id uuid, school_id uuid, initiated_by uuid, kind text,
			file_name text, rows_json text, errors_json text, attempts integer,
			worker_id uuid, claimed_at timestamptz, last_error text)
		LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog AS $fn$
			UPDATE ` + table + ` AS j
			SET status = 'running', worker_id = p_worker,
				claimed_at = clock_timestamp(), attempts = j.attempts + 1, last_error = ''
			WHERE j.id = (
				SELECT q.id FROM ` + table + ` AS q
				WHERE q.status = 'queued'
				   OR (q.status = 'running' AND (q.claimed_at IS NULL OR q.claimed_at < clock_timestamp() - interval '5 minutes'))
				ORDER BY q.created_at, q.id
				LIMIT 1
				FOR UPDATE SKIP LOCKED
			)
			RETURNING j.id, j.school_id, j.initiated_by, j.kind,
				j.file_name, j.rows_json, j.errors_json, j.attempts,
				j.worker_id, j.claimed_at, j.last_error
		$fn$`
	if err := admin.Exec("GRANT CREATE ON SCHEMA " + schema + " TO " + ownerRole).Error; err != nil {
		t.Fatalf("temporarily grant function creation: %v", err)
	}
	if err := admin.Exec(createFunction).Error; err != nil {
		t.Fatalf("create isolated definer fixture: %v", err)
	}
	// Revoke the default PUBLIC EXECUTE privilege while the creating role
	// still owns the function. Do this before ownership transfer so the
	// unrelated login cannot inherit the default function privilege.
	if err := admin.Exec("REVOKE ALL ON FUNCTION " + function + "(uuid) FROM PUBLIC").Error; err != nil {
		t.Fatalf("revoke public function execution: %v", err)
	}
	// The creating identity owns the function at this point, so it can
	// establish the least-privilege ACL before transferring ownership.
	if err := admin.Exec("GRANT EXECUTE ON FUNCTION " + function + "(uuid) TO " + runtimeRole).Error; err != nil {
		t.Fatalf("grant runtime function execution: %v", err)
	}
	if err := admin.Exec("GRANT " + ownerRole + " TO " + identity.Role).Error; err != nil {
		t.Fatalf("temporarily grant function-owner membership: %v", err)
	}
	if err := admin.Exec("ALTER FUNCTION " + function + "(uuid) OWNER TO " + ownerRole).Error; err != nil {
		t.Fatalf("assign definer owner: %v", err)
	}
	if err := admin.Exec("REVOKE " + ownerRole + " FROM " + identity.Role).Error; err != nil {
		t.Fatalf("revoke temporary function-owner membership: %v", err)
	}
	if err := admin.Exec("REVOKE CREATE ON SCHEMA " + schema + " FROM " + ownerRole).Error; err != nil {
		t.Fatalf("revoke temporary schema create: %v", err)
	}

	runtimeDSN := *parsed
	runtimeDSN.User = url.UserPassword(runtimeLogin, runtimePassword)
	query := runtimeDSN.Query()
	query.Set("search_path", schema)
	runtimeDSN.RawQuery = query.Encode()
	runtimeDB, err := gorm.Open(postgres.Open(runtimeDSN.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect as restricted runtime login: %v", err)
	}
	runtimeSQL, err := runtimeDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeSQL.Close()

	var bypass bool
	if err := runtimeDB.Raw("SELECT rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(&bypass).Error; err != nil {
		t.Fatalf("inspect runtime RLS attribute: %v", err)
	}
	if bypass {
		t.Fatal("restricted runtime login unexpectedly has BYPASSRLS")
	}
	for _, statement := range []string{
		"SELECT * FROM " + table,
		"UPDATE " + table + " SET status = 'failed'",
		"INSERT INTO " + table + " (id, school_id, initiated_by, kind, file_name, status, rows_json, errors_json, created_at) VALUES (gen_random_uuid(), gen_random_uuid(), gen_random_uuid(), 'students', 'x.csv', 'queued', '[]', '[]', now())",
		"DELETE FROM " + table,
	} {
		if err := runtimeDB.Exec(statement).Error; err == nil {
			t.Errorf("runtime unexpectedly had direct queue-table access: %s", statement)
		}
	}

	workerID := uuid.New()
	var claimed []restrictedClaimRow
	if err := runtimeDB.Raw("SELECT * FROM " + function + "(?)", workerID).Scan(&claimed).Error; err != nil {
		t.Fatalf("authorized runtime function call: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != jobID || claimed[0].Attempts != 3 {
		t.Fatalf("function did not reclaim the oldest expired job with attempts=3: %+v", claimed)
	}
	if claimed[0].WorkerID == nil || *claimed[0].WorkerID != workerID || claimed[0].ClaimedAt == nil || claimed[0].LastError != "" {
		t.Fatalf("claim did not refresh worker/lease and clear prior error: %+v", claimed[0])
	}
	if claimed[0].SchoolID != schoolID || claimed[0].InitiatedBy != actorID || claimed[0].RowsJSON != `[{"name":"Expired payload"}]` || claimed[0].ErrorsJSON != `[{"row":2,"message":"previous"}]` {
		t.Fatalf("claim did not preserve tenant, initiator, and payload fields: %+v", claimed[0])
	}
	claimed = nil
	if err := runtimeDB.Raw("SELECT * FROM " + function + "(?)", workerID).Scan(&claimed).Error; err != nil {
		t.Fatalf("claim queued job: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != queuedID || claimed[0].Attempts != 1 || claimed[0].LastError != "" {
		t.Fatalf("expected oldest queued job with attempts=1 and cleared error: %+v", claimed)
	}
	claimed = nil
	if err := runtimeDB.Raw("SELECT * FROM " + function + "(?)", workerID).Scan(&claimed).Error; err != nil {
		t.Fatalf("claim unleased running job: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != unleasedID || claimed[0].Attempts != 5 {
		t.Fatalf("expected unleased running job recovery with attempts=5: %+v", claimed)
	}
	claimed = nil
	if err := runtimeDB.Raw("SELECT * FROM " + function + "(?)", workerID).Scan(&claimed).Error; err != nil {
		t.Fatalf("check exhausted queue: %v", err)
	}
	if len(claimed) != 0 {
		t.Fatalf("active lease must not be reclaimed and no eligible jobs remain: %+v", claimed)
	}

	// Verify the catalog-level function boundary, not just its observed behavior.
	var metadata struct {
		Owner string
		SecurityDefiner bool
		SearchPath string
		RuntimeCanExecute bool
		OutsiderCanExecute bool
		PublicCanExecute bool
	}
	if err := admin.Raw(`SELECT pg_get_userbyid(p.proowner) AS owner,
		p.prosecdef AS security_definer,
		COALESCE(array_to_string(p.proconfig, ','), '') AS search_path,
		has_function_privilege(?, p.oid, 'EXECUTE') AS runtime_can_execute,
		has_function_privilege(?, p.oid, 'EXECUTE') AS outsider_can_execute,
		EXISTS (SELECT 1 FROM aclexplode(COALESCE(p.proacl, acldefault('f', p.proowner))) acl
			WHERE acl.grantee = 0 AND acl.privilege_type = 'EXECUTE') AS public_can_execute
		FROM pg_proc p WHERE p.oid = ?::regprocedure`,
		runtimeLogin, outsiderLogin, function+"(uuid)").Scan(&metadata).Error; err != nil {
		t.Fatalf("inspect restricted function metadata: %v", err)
	}
	if metadata.Owner != ownerRole || !metadata.SecurityDefiner || metadata.SearchPath != "search_path=pg_catalog" {
		t.Fatalf("unexpected function security metadata: %+v", metadata)
	}
	if !metadata.RuntimeCanExecute || metadata.OutsiderCanExecute || metadata.PublicCanExecute {
		t.Fatalf("unexpected function execution ACL: %+v", metadata)
	}

	// Hold the oldest queue row locked from a separate connection. Concurrent
	// restricted callers must skip it and claim different eligible rows.
	concurrentIDs := make([]uuid.UUID, 5)
	for i := range concurrentIDs {
		concurrentIDs[i] = uuid.New()
		if err := admin.Exec(seed, concurrentIDs[i], schoolID, actorID,
			fmt.Sprintf("concurrent-%d.csv", i), "queued", "[]", "[]",
			now.Add(time.Duration(i)*time.Second), nil, 0, "").Error; err != nil {
			t.Fatalf("seed concurrent queue job %d: %v", i, err)
		}
	}
	lockTx := admin.Begin()
	if lockTx.Error != nil {
		t.Fatalf("begin queue row lock transaction: %v", lockTx.Error)
	}
	var lockedID uuid.UUID
	if err := lockTx.Raw("SELECT id FROM "+table+" WHERE id = ? FOR UPDATE", concurrentIDs[0]).Scan(&lockedID).Error; err != nil {
		_ = lockTx.Rollback().Error
		t.Fatalf("lock oldest concurrent job: %v", err)
	}
	if lockedID != concurrentIDs[0] {
		_ = lockTx.Rollback().Error
		t.Fatalf("failed to lock intended oldest job: got %s want %s", lockedID, concurrentIDs[0])
	}

	type claimResult struct {
		row restrictedClaimRow
		err error
	}
	results := make(chan claimResult, 3)
	start := make(chan struct{})
	for i := 0; i < 3; i++ {
		go func() {
			<-start
			var rows []restrictedClaimRow
			worker := uuid.New()
			err := runtimeDB.Raw("SELECT * FROM "+function+"(?)", worker).Scan(&rows).Error
			if err != nil {
				results <- claimResult{err: err}
				return
			}
			if len(rows) != 1 {
				results <- claimResult{err: fmt.Errorf("concurrent claim returned %d rows, want 1", len(rows))}
				return
			}
			if rows[0].WorkerID == nil || *rows[0].WorkerID != worker {
				results <- claimResult{err: fmt.Errorf("concurrent claim did not persist caller worker ID")}
				return
			}
			results <- claimResult{row: rows[0]}
		}()
	}
	close(start)
	claimedIDs := make(map[uuid.UUID]bool)
	for i := 0; i < 3; i++ {
		result := <-results
		if result.err != nil {
			_ = lockTx.Rollback().Error
			t.Fatalf("concurrent restricted claim: %v", result.err)
		}
		if result.row.ID == concurrentIDs[0] {
			_ = lockTx.Rollback().Error
			t.Fatal("restricted claim did not skip the externally locked oldest job")
		}
		if claimedIDs[result.row.ID] {
			_ = lockTx.Rollback().Error
			t.Fatalf("two concurrent callers claimed the same job %s", result.row.ID)
		}
		claimedIDs[result.row.ID] = true
	}
	if err := lockTx.Commit().Error; err != nil {
		t.Fatalf("release oldest queue row lock: %v", err)
	}
	var finalClaim []restrictedClaimRow
	if err := runtimeDB.Raw("SELECT * FROM "+function+"(?)", uuid.New()).Scan(&finalClaim).Error; err != nil {
		t.Fatalf("claim previously locked job after release: %v", err)
	}
	if len(finalClaim) != 1 || finalClaim[0].ID != concurrentIDs[0] {
		t.Fatalf("expected oldest locked job to become claimable after release, got %+v", finalClaim)
	}

	outsiderDSN := *parsed
	outsiderDSN.User = url.UserPassword(outsiderLogin, outsiderPassword)
	outsiderDSN.RawQuery = query.Encode()
	outsiderDB, err := gorm.Open(postgres.Open(outsiderDSN.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect unrelated login: %v", err)
	}
	outsiderSQL, err := outsiderDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer outsiderSQL.Close()
	var outsiderRows []restrictedClaimRow
	if err := outsiderDB.Raw("SELECT * FROM " + function + "(?)", uuid.New()).Scan(&outsiderRows).Error; err == nil {
		t.Fatal("unrelated login unexpectedly executed the restricted claim function")
	}

	var directPrivileges int
	if err := admin.Raw(`SELECT count(*) FROM information_schema.role_table_grants
		WHERE grantee IN (?, ?) AND table_schema = ? AND table_name = 'bulk_import_jobs'`,
		runtimeLogin, runtimeRole, schema).Scan(&directPrivileges).Error; err != nil {
		t.Fatalf("inspect runtime direct grants: %v", err)
	}
	if directPrivileges != 0 {
		t.Fatalf("runtime login has %d direct table grants; expected none", directPrivileges)
	}

	var ownerIsTableOwner bool
	if err := admin.Raw(`SELECT pg_get_userbyid(c.relowner) = ? FROM pg_class c WHERE c.oid = ?::regclass`, ownerRole, table).Scan(&ownerIsTableOwner).Error; err != nil {
		t.Fatalf("inspect queue table ownership: %v", err)
	}
	if ownerIsTableOwner {
		t.Fatal("queue function owner must not own the queue table")
	}

	var privilegedRoles int
	if err := admin.Raw(`SELECT count(*) FROM pg_roles WHERE rolname IN (?, ?, ?) AND (rolsuper OR rolbypassrls)`, ownerRole, runtimeRole, runtimeLogin).Scan(&privilegedRoles).Error; err != nil {
		t.Fatalf("inspect restricted role attributes: %v", err)
	}
	if privilegedRoles != 0 {
		t.Fatalf("%d restricted queue roles unexpectedly have SUPERUSER or BYPASSRLS", privilegedRoles)
	}
	var ownerMembership bool
	if err := admin.Raw(`SELECT pg_has_role(?, ?, 'MEMBER')`, runtimeLogin, ownerRole).Scan(&ownerMembership).Error; err != nil {
		t.Fatalf("inspect runtime/function-owner membership: %v", err)
	}
	if ownerMembership {
		t.Fatal("runtime login must not be a member of the function-owner role")
	}
}
