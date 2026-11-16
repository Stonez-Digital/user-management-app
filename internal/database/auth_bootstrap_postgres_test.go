package database

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAuthBootstrapFunctionsEnforceRestrictedPostgresBoundary(t *testing.T) {
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
		t.Fatalf("connect PostgreSQL test administrator: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer adminSQL.Close()

	var canCreateRoles bool
	if err := admin.Raw("SELECT rolsuper OR rolcreaterole FROM pg_roles WHERE rolname = current_user").Scan(&canCreateRoles).Error; err != nil || !canCreateRoles {
		t.Skip("TEST_DATABASE_URL requires CREATEROLE to exercise restricted auth roles")
	}

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	tableSchema := "auth_bootstrap_" + suffix
	privateSchema := "auth_internal_" + suffix
	ownerRole := "auth_owner_" + suffix
	sessionOwnerRole := "auth_session_owner_" + suffix
	apiRole := "auth_api_" + suffix
	otherRole := "auth_other_" + suffix
	loginRole := "auth_login_" + suffix
	loginPassword := uuid.NewString()

	if err := admin.Exec("CREATE SCHEMA " + tableSchema).Error; err != nil {
		t.Fatalf("create isolated auth schema: %v", err)
	}
	cleanup := func() {
		_ = admin.Exec("REVOKE " + apiRole + " FROM " + loginRole).Error
		_ = admin.Exec("DROP SCHEMA IF EXISTS " + privateSchema + " CASCADE").Error
		_ = admin.Exec("DROP SCHEMA IF EXISTS " + tableSchema + " CASCADE").Error
		for _, role := range []string{loginRole, otherRole, apiRole, ownerRole, sessionOwnerRole} {
			_ = admin.Exec("DROP OWNED BY " + role).Error
			_ = admin.Exec("REVOKE " + role + " FROM CURRENT_USER").Error
			_ = admin.Exec("DROP ROLE IF EXISTS " + role).Error
		}
	}
	defer cleanup()

	for _, statement := range []string{
		"CREATE ROLE " + ownerRole + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS",
		"CREATE ROLE " + apiRole + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS",
		"CREATE ROLE " + otherRole + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS",
		"CREATE ROLE " + loginRole + " LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS PASSWORD '" + loginPassword + "'",
		"GRANT " + ownerRole + ", " + apiRole + ", " + otherRole + " TO CURRENT_USER",
		"GRANT " + apiRole + " TO " + loginRole,
		"CREATE SCHEMA " + privateSchema,
	} {
		if err := admin.Exec(statement).Error; err != nil {
			t.Fatalf("prepare isolated auth role/schema (%s): %v", statement, err)
		}
	}

	tables := []string{
		"CREATE TABLE " + tableSchema + ".schools (id uuid PRIMARY KEY, status text NOT NULL, code text NOT NULL, slug text NOT NULL)",
		"CREATE TABLE " + tableSchema + ".users (id uuid PRIMARY KEY, school_id uuid, name text NOT NULL, email text NOT NULL, password_hash text NOT NULL, role text NOT NULL, active boolean NOT NULL)",
		"CREATE TABLE " + tableSchema + ".password_reset_tokens (id uuid PRIMARY KEY, user_id uuid NOT NULL, token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL, used boolean NOT NULL, created_at timestamptz)",
		"CREATE TABLE " + tableSchema + ".refresh_tokens (id uuid PRIMARY KEY, family_id uuid NOT NULL, user_id uuid NOT NULL, token_hash text NOT NULL, expires_at timestamptz NOT NULL, revoked boolean NOT NULL)",
		"CREATE TABLE " + tableSchema + ".sessions (id uuid PRIMARY KEY, user_id uuid NOT NULL, family_id uuid NOT NULL, refresh_token_hash text NOT NULL, expires_at timestamptz NOT NULL, revoked boolean NOT NULL, user_agent text, ip text)",
	}
	for _, statement := range tables {
		if err := admin.Exec(statement).Error; err != nil {
			t.Fatalf("create isolated auth table: %v", err)
		}
	}
	if err := admin.Exec("GRANT USAGE ON SCHEMA " + tableSchema + " TO " + apiRole).Error; err != nil {
		t.Fatalf("grant API role schema visibility for table-level denial checks: %v", err)
	}
	for _, table := range []string{"schools", "users", "refresh_tokens", "sessions"} {
		if err := admin.Exec("ALTER TABLE " + tableSchema + "." + table + " ENABLE ROW LEVEL SECURITY").Error; err != nil {
			t.Fatalf("enable RLS on %s: %v", table, err)
		}
		if err := admin.Exec("ALTER TABLE " + tableSchema + "." + table + " FORCE ROW LEVEL SECURITY").Error; err != nil {
			t.Fatalf("force RLS on %s: %v", table, err)
		}
	}

	schoolID, userID := uuid.New(), uuid.New()
	familyID, refreshID, sessionID := uuid.New(), uuid.New(), uuid.New()
	tokenHash := "f00ba4f00ba4f00ba4f00ba4f00ba4f00ba4f00ba4f00ba4f00ba4f00ba4f00ba4"
	expires := "2099-01-01T00:00:00Z"
	for _, statement := range []string{
		fmt.Sprintf("INSERT INTO %s.schools VALUES ('%s','active','SCH-A','school-a')", tableSchema, schoolID),
		fmt.Sprintf("INSERT INTO %s.users VALUES ('%s','%s','Test User','test@example.com','bcrypt-hash','school_admin',true)", tableSchema, userID, schoolID),
		fmt.Sprintf("INSERT INTO %s.refresh_tokens VALUES ('%s','%s','%s','%s','%s',false)", tableSchema, refreshID, familyID, userID, tokenHash, expires),
		fmt.Sprintf("INSERT INTO %s.sessions VALUES ('%s','%s','%s','%s','%s',false,NULL,NULL)", tableSchema, sessionID, userID, familyID, tokenHash, expires),
	} {
		if err := admin.Exec(statement).Error; err != nil {
			t.Fatalf("seed isolated auth fixture: %v", err)
		}
	}

	readScript := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile("../../scripts/security/" + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		script := string(raw)
		script = strings.ReplaceAll(script, "stonez_auth_bootstrap_owner", ownerRole)
		script = strings.ReplaceAll(script, "stonez_auth_session_owner", sessionOwnerRole)
		script = strings.ReplaceAll(script, "stonez_api_runtime", apiRole)
		script = strings.ReplaceAll(script, "stonez_internal", privateSchema)
		script = strings.ReplaceAll(script, "public", tableSchema)
		return script
	}
	for _, name := range []string{
		"provision_auth_bootstrap_roles.sql",
		"install_auth_bootstrap_functions.sql",
		"verify_auth_bootstrap_boundary.sql",
	} {
		if err := admin.Exec(readScript(name)).Error; err != nil {
			t.Fatalf("execute shipped auth bootstrap script %s: %v", name, err)
		}
	}
	if err := admin.Exec("GRANT USAGE ON SCHEMA " + privateSchema + " TO " + otherRole).Error; err != nil {
		t.Fatalf("grant unrelated role schema visibility: %v", err)
	}

	loginURL := *parsed
	loginURL.User = url.UserPassword(loginRole, loginPassword)
	loginDB, err := gorm.Open(postgres.Open(loginURL.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect as restricted API login: %v", err)
	}
	loginSQL, err := loginDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer loginSQL.Close()

	type loginResult struct {
		UserID uuid.UUID
		SchoolID uuid.UUID
		FullName string
		Email string
		PasswordHash string
		Role string
		Active bool
		SchoolStatus string
		SchoolCode string
		SchoolSlug string
	}
	var login loginResult
	if err := loginDB.Raw("SELECT * FROM " + privateSchema + ".auth_login_lookup(?)", " TEST@example.com ").Scan(&login).Error; err != nil {
		t.Fatalf("execute login lookup as restricted API login: %v", err)
	}
	if login.UserID != userID || login.SchoolID != schoolID || login.PasswordHash != "bcrypt-hash" ||
		login.SchoolStatus != "active" || login.SchoolCode != "SCH-A" || login.SchoolSlug != "school-a" {
		t.Fatalf("login lookup returned unexpected identity/school data: %+v", login)
	}

	type refreshResult struct {
		TokenID uuid.UUID
		FamilyID uuid.UUID
		UserID uuid.UUID
		SessionID uuid.UUID
		SchoolID uuid.UUID
		Active bool
		SchoolStatus string
	}
	var refresh refreshResult
	if err := loginDB.Raw("SELECT token_id, family_id, user_id, session_id, school_id, active, school_status FROM " + privateSchema + ".auth_refresh_lookup(?)", tokenHash).Scan(&refresh).Error; err != nil {
		t.Fatalf("execute refresh lookup as restricted API login: %v", err)
	}
	if refresh.TokenID != refreshID || refresh.FamilyID != familyID || refresh.UserID != userID ||
		refresh.SessionID != sessionID || refresh.SchoolID != schoolID || !refresh.Active || refresh.SchoolStatus != "active" {
		t.Fatalf("refresh lookup returned unexpected token/session/identity data: %+v", refresh)
	}

	newFamilyID, newRefreshID, newSessionID := uuid.New(), uuid.New(), uuid.New()
	newHash := "new-refresh-hash-" + suffix
	newExpiry, err := time.Parse(time.RFC3339, "2099-02-01T00:00:00Z")
	if err != nil { t.Fatal(err) }
	expiresAt, err := time.Parse(time.RFC3339, expires)
	if err != nil { t.Fatal(err) }
	if err := loginDB.Exec("SELECT " + privateSchema + ".auth_create_session(?, ?, ?, ?, ?, ?, ?, ?)",
		newRefreshID, userID, newFamilyID, newHash, expiresAt, newSessionID, "fixture-agent", "127.0.0.1").Error; err != nil {
		t.Fatalf("create session through restricted writer function: %v", err)
	}
	if err := loginDB.Exec("INSERT INTO " + tableSchema + ".refresh_tokens (id,family_id,user_id,token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)",
		uuid.New(), newFamilyID, userID, "direct-write-denied", expires).Error; err == nil {
		t.Fatal("API runtime unexpectedly inserted a refresh token directly")
	}
	var rotated bool
	if err := loginDB.Raw("SELECT " + privateSchema + ".auth_rotate_refresh(?, ?, ?, ?, ?, ?, ?, ?)",
		newRefreshID, newHash, uuid.New(), userID, newFamilyID, "rotated-"+suffix, newExpiry, newSessionID).Scan(&rotated).Error; err != nil || !rotated {
		t.Fatalf("rotate refresh through restricted writer function: rotated=%v err=%v", rotated, err)
	}
	if err := loginDB.Exec("SELECT " + privateSchema + ".auth_revoke_refresh_family(?)", newFamilyID).Error; err != nil {
		t.Fatalf("revoke refresh family through restricted writer function: %v", err)
	}

	// A missing/mismatched session must return false without revoking the old
	// token or inserting a replacement, even when the function is called alone.
	atomicFamily, atomicToken, atomicSession := uuid.New(), uuid.New(), uuid.New()
	atomicHash := "atomic-old-" + suffix
	if err := loginDB.Exec("SELECT " + privateSchema + ".auth_create_session(?, ?, ?, ?, ?, ?, ?, ?)",
		atomicToken, userID, atomicFamily, atomicHash, expiresAt, atomicSession, "fixture-agent", "127.0.0.1").Error; err != nil {
		t.Fatalf("create atomicity fixture: %v", err)
	}
	if err := admin.Exec("DELETE FROM " + tableSchema + ".sessions WHERE id = ?", atomicSession).Error; err != nil {
		t.Fatalf("remove session for atomicity test: %v", err)
	}
	var shouldNotRotate bool
	if err := loginDB.Raw("SELECT " + privateSchema + ".auth_rotate_refresh(?, ?, ?, ?, ?, ?, ?, ?)",
		atomicToken, atomicHash, uuid.New(), userID, atomicFamily, "atomic-new-"+suffix, newExpiry, atomicSession).Scan(&shouldNotRotate).Error; err != nil {
		t.Fatalf("run missing-session rotation: %v", err)
	}
	if shouldNotRotate {
		t.Fatal("rotation unexpectedly succeeded without a matching session")
	}
	var oldStillActive int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("id = ? AND revoked = false", atomicToken).Count(&oldStillActive).Error; err != nil {
		t.Fatal(err)
	}
	if oldStillActive != 1 {
		t.Fatal("failed rotation changed the old token despite a missing session")
	}
	var unexpectedReplacement int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("family_id = ? AND token_hash = ?", atomicFamily, "atomic-new-"+suffix).Count(&unexpectedReplacement).Error; err != nil {
		t.Fatal(err)
	}
	if unexpectedReplacement != 0 {
		t.Fatal("failed rotation inserted a replacement token")
	}

	// Two independent API connections racing the same refresh may produce
	// exactly one replacement. The loser must observe the changed session hash.
	raceFamily, raceToken, raceSession := uuid.New(), uuid.New(), uuid.New()
	raceHash := "race-old-" + suffix
	if err := loginDB.Exec("SELECT " + privateSchema + ".auth_create_session(?, ?, ?, ?, ?, ?, ?, ?)",
		raceToken, userID, raceFamily, raceHash, expiresAt, raceSession, "fixture-agent", "127.0.0.1").Error; err != nil {
		t.Fatalf("create concurrency fixture: %v", err)
	}
	secondLoginDB, err := gorm.Open(postgres.Open(loginURL.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open second restricted API connection: %v", err)
	}
	secondSQL, err := secondLoginDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer secondSQL.Close()
	start := make(chan struct{})
	results := make(chan bool, 2)
	var wg sync.WaitGroup
	for i, db := range []*gorm.DB{loginDB, secondLoginDB} {
		wg.Add(1)
		go func(i int, db *gorm.DB) {
			defer wg.Done()
			<-start
			var ok bool
			newHash := fmt.Sprintf("race-new-%s-%d", suffix, i)
			err := db.Raw("SELECT " + privateSchema + ".auth_rotate_refresh(?, ?, ?, ?, ?, ?, ?, ?)",
				raceToken, raceHash, uuid.New(), userID, raceFamily, newHash, newExpiry, raceSession).Scan(&ok).Error
			if err != nil {
				t.Errorf("concurrent rotation %d: %v", i, err)
				return
			}
			results <- ok
		}(i, db)
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for ok := range results {
		if ok { successes++ }
	}
	if successes != 1 {
		t.Fatalf("expected exactly one concurrent refresh rotation to succeed, got %d", successes)
	}
	var raceActive int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("family_id = ? AND revoked = false", raceFamily).Count(&raceActive).Error; err != nil {
		t.Fatal(err)
	}
	if raceActive != 1 {
		t.Fatalf("expected one active token after concurrent rotation, got %d", raceActive)
	}

	err = admin.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + sessionOwnerRole).Error; err != nil {
			return err
		}
		return tx.Exec("SELECT password_hash FROM " + tableSchema + ".users").Error
	})
	if err == nil {
		t.Fatal("session writer owner unexpectedly read user credentials")
	}

	for _, statement := range []string{
		"SELECT * FROM " + tableSchema + ".users",
		"SELECT * FROM " + tableSchema + ".refresh_tokens",
		"DELETE FROM " + tableSchema + ".sessions",
		"SET ROLE " + ownerRole,
	} {
		if err := loginDB.Exec(statement).Error; err == nil {
			t.Fatalf("restricted API login unexpectedly executed %q", statement)
		}
	}

	err = admin.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + ownerRole).Error; err != nil {
			return err
		}
		var visible []uuid.UUID
		if err := tx.Table(tableSchema + ".users").Pluck("id", &visible).Error; err != nil {
			return err
		}
		if len(visible) != 1 || visible[0] != userID {
			return fmt.Errorf("bootstrap owner expected the seeded user only, got %v", visible)
		}
		return tx.Exec("UPDATE " + tableSchema + ".users SET active = false").Error
	})
	if err == nil {
		t.Fatal("bootstrap owner unexpectedly updated user records")
	}

	err = admin.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SET LOCAL ROLE " + otherRole).Error; err != nil {
			return err
		}
		return tx.Exec("SELECT * FROM " + privateSchema + ".auth_login_lookup('test@example.com')").Error
	})
	if err == nil {
		t.Fatal("unrelated role unexpectedly executed auth bootstrap function")
	}
}
