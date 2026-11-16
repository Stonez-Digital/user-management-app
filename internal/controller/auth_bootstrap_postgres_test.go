package controller

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/auth"
	"github.com/onoja217/users-management-app/internal/authz"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This exercises the real PostgreSQL SECURITY DEFINER lookup functions through
// the HTTP handlers while the handler DB connection has only runtime grants.
func TestPostgresAuthBootstrapLoginAndRefreshHandlers(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect isolated PostgreSQL administrator: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer adminSQL.Close()

	var canCreateRoles bool
	if err := admin.Raw("SELECT rolsuper OR rolcreaterole FROM pg_roles WHERE rolname = current_user").Scan(&canCreateRoles).Error; err != nil || !canCreateRoles {
		t.Skip("TEST_DATABASE_URL requires CREATEROLE to exercise restricted auth handlers")
	}

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	tableSchema, privateSchema := "auth_handler_"+suffix, "stonez_internal"
	ownerRole, sessionOwnerRole, apiRole := "auth_handler_owner_"+suffix, "auth_handler_session_owner_"+suffix, "auth_handler_api_"+suffix
	loginRole := "auth_handler_login_"+suffix
	loginPassword := uuid.NewString()
	if err := admin.Exec("CREATE SCHEMA " + tableSchema).Error; err != nil {
		t.Fatalf("create isolated handler schema: %v", err)
	}
	cleanup := func() {
		_ = admin.Exec("REVOKE " + apiRole + " FROM " + loginRole).Error
		_ = admin.Exec("DROP SCHEMA IF EXISTS " + privateSchema + " CASCADE").Error
		_ = admin.Exec("DROP SCHEMA IF EXISTS " + tableSchema + " CASCADE").Error
		for _, role := range []string{loginRole, apiRole, ownerRole, sessionOwnerRole} {
			_ = admin.Exec("DROP OWNED BY " + role).Error
			_ = admin.Exec("REVOKE " + role + " FROM CURRENT_USER").Error
			_ = admin.Exec("DROP ROLE IF EXISTS " + role).Error
		}
	}
	defer cleanup()

	for _, statement := range []string{
		"CREATE ROLE " + ownerRole + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS",
		"CREATE ROLE " + apiRole + " NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS",
		"CREATE ROLE " + loginRole + " LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE INHERIT NOBYPASSRLS PASSWORD '" + loginPassword + "'",
		"GRANT " + ownerRole + ", " + apiRole + " TO CURRENT_USER",
		"GRANT " + apiRole + " TO " + loginRole,
		"CREATE SCHEMA " + privateSchema,
	} {
		if err := admin.Exec(statement).Error; err != nil {
			t.Fatalf("provision isolated handler roles: %v", err)
		}
	}

	statements := []string{
		"CREATE TABLE " + tableSchema + ".schools (id uuid PRIMARY KEY, status text NOT NULL, code text NOT NULL, slug text NOT NULL)",
		"CREATE TABLE " + tableSchema + ".users (id uuid PRIMARY KEY, school_id uuid, name text NOT NULL, email text NOT NULL, password_hash text NOT NULL, role text NOT NULL, active boolean NOT NULL)",
		"CREATE TABLE " + tableSchema + ".refresh_tokens (id uuid PRIMARY KEY, family_id uuid NOT NULL, user_id uuid NOT NULL, token_hash text NOT NULL, expires_at timestamptz NOT NULL, revoked boolean NOT NULL, created_at timestamptz)",
		"CREATE TABLE " + tableSchema + ".sessions (id uuid PRIMARY KEY, user_id uuid NOT NULL, family_id uuid NOT NULL, refresh_token_hash text NOT NULL, expires_at timestamptz NOT NULL, revoked boolean NOT NULL, user_agent text, ip text, created_at timestamptz)",
		"CREATE TABLE " + tableSchema + ".audit_logs (id uuid PRIMARY KEY, school_id uuid, actor_id uuid, action text NOT NULL, resource text NOT NULL, resource_id uuid, ip text, metadata text, created_at timestamptz)",
	}
	for _, statement := range statements {
		if err := admin.Exec(statement).Error; err != nil {
			t.Fatalf("create handler fixture tables: %v", err)
		}
	}
	for _, statement := range []string{
		"GRANT USAGE ON SCHEMA " + tableSchema + " TO " + apiRole,
		"GRANT INSERT ON " + tableSchema + ".audit_logs TO " + apiRole,
			} {
		if err := admin.Exec(statement).Error; err != nil {
			t.Fatalf("grant restricted handler privileges: %v", err)
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
		return strings.ReplaceAll(script, "public", tableSchema)
	}
	for _, name := range []string{"provision_auth_bootstrap_roles.sql", "install_auth_bootstrap_functions.sql", "verify_auth_bootstrap_boundary.sql"} {
		if err := admin.Exec(readScript(name)).Error; err != nil {
			t.Fatalf("install auth bootstrap fixture script %s: %v", name, err)
		}
	}

	loginDSN, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse test database URL: %v", err)
	}
	loginDSN.User = url.UserPassword(loginRole, loginPassword)
	query := loginDSN.Query()
	query.Set("options", "-csearch_path="+tableSchema)
	loginDSN.RawQuery = query.Encode()
	runtimeDB, err := gorm.Open(postgres.Open(loginDSN.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect as restricted API login: %v", err)
	}
	runtimeSQL, err := runtimeDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer runtimeSQL.Close()

	if err := auth.ConfigureSecret("test-auth-secret-for-controller-tests-32chars"); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	schoolID, userID := uuid.New(), uuid.New()
	passwordHash, err := auth.HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	email := "handler-" + suffix + "@example.test"
	if err := admin.Exec(fmt.Sprintf("INSERT INTO %s.schools VALUES ('%s','active','SCH-%s','school-%s')", tableSchema, schoolID, suffix, suffix)).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec(fmt.Sprintf("INSERT INTO %s.users VALUES ('%s','%s','Handler User','%s','%s','%s',true)", tableSchema, userID, schoolID, email, passwordHash, authz.RoleSchoolAdmin)).Error; err != nil {
		t.Fatal(err)
	}

	ac := NewAuthController(runtimeDB)
	loginBody := fmt.Sprintf(`{"email":%q,"password":"password123","school_code":%q,"school_slug":%q}`, email, "SCH-"+suffix, "school-"+suffix)

	// Login session creation and its audit event must commit or roll back together.
	if err := admin.Exec("CREATE FUNCTION " + privateSchema + ".reject_auth_audit() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RAISE EXCEPTION ''audit sink unavailable''; END'").Error; err != nil {
		t.Fatalf("create audit failure trigger function: %v", err)
	}
	if err := admin.Exec("CREATE TRIGGER reject_auth_audit BEFORE INSERT ON " + tableSchema + ".audit_logs FOR EACH ROW EXECUTE FUNCTION " + privateSchema + ".reject_auth_audit()").Error; err != nil {
		t.Fatalf("install audit failure trigger: %v", err)
	}
	failedLogin := authJSONRequest(t, ac, "POST", "/login", loginBody)
	if failedLogin.Code != 500 {
		t.Fatalf("expected login to fail when audit insert fails, got %d: %s", failedLogin.Code, failedLogin.Body.String())
	}
	var orphanedSessions int64
	if err := admin.Table(tableSchema + ".sessions").Where("user_id = ?", userID).Count(&orphanedSessions).Error; err != nil {
		t.Fatal(err)
	}
	if orphanedSessions != 0 {
		t.Fatalf("login audit failure left %d persisted sessions", orphanedSessions)
	}
	if err := admin.Exec("DROP TRIGGER reject_auth_audit ON " + tableSchema + ".audit_logs").Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("DROP FUNCTION " + privateSchema + ".reject_auth_audit()").Error; err != nil {
		t.Fatal(err)
	}

	_, rawRefresh := decodeAuthTokens(t, authJSONRequest(t, ac, "POST", "/login", loginBody))
	var loginAuditCount int64
	if err := admin.Table(tableSchema + ".audit_logs").Where("action = ? AND resource_id = ?", "auth.login", userID).Count(&loginAuditCount).Error; err != nil {
		t.Fatal(err)
	}
	if loginAuditCount != 1 {
		t.Fatalf("expected exactly one committed login audit event, got %d", loginAuditCount)
	}

	// Logout must use the restricted writer and commit its audit event atomically.
	_, logoutRefresh := decodeAuthTokens(t, authJSONRequest(t, ac, "POST", "/login", loginBody))
	logoutHash := auth.HashRefreshToken(logoutRefresh)
	logoutResponse := authJSONRequest(t, ac, "POST", "/logout", fmt.Sprintf(`{"refresh_token":%q}`, logoutRefresh))
	if logoutResponse.Code != 200 {
		t.Fatalf("expected restricted PostgreSQL logout to succeed, got %d: %s", logoutResponse.Code, logoutResponse.Body.String())
	}
	var loggedOutTokens, loggedOutSessions, logoutAuditCount int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("token_hash = ? AND revoked = true", logoutHash).Count(&loggedOutTokens).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Table(tableSchema + ".sessions").Where("refresh_token_hash = ? AND revoked = true", logoutHash).Count(&loggedOutSessions).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Table(tableSchema + ".audit_logs").Where("action = ? AND resource_id = ?", "auth.logout", userID).Count(&logoutAuditCount).Error; err != nil {
		t.Fatal(err)
	}
	if loggedOutTokens != 1 || loggedOutSessions != 1 || logoutAuditCount != 1 {
		t.Fatalf("logout did not atomically revoke its token/session and audit: tokens=%d sessions=%d audits=%d", loggedOutTokens, loggedOutSessions, logoutAuditCount)
	}

	// An audit failure must roll back both logout revocation writes.
	_, failedLogoutRefresh := decodeAuthTokens(t, authJSONRequest(t, ac, "POST", "/login", loginBody))
	failedLogoutHash := auth.HashRefreshToken(failedLogoutRefresh)
	if err := admin.Exec("CREATE FUNCTION " + privateSchema + ".reject_auth_audit() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RAISE EXCEPTION ''audit sink unavailable''; END'").Error; err != nil {
		t.Fatalf("create logout audit failure trigger function: %v", err)
	}
	if err := admin.Exec("CREATE TRIGGER reject_auth_audit BEFORE INSERT ON " + tableSchema + ".audit_logs FOR EACH ROW EXECUTE FUNCTION " + privateSchema + ".reject_auth_audit()").Error; err != nil {
		t.Fatalf("install logout audit failure trigger: %v", err)
	}
	failedLogout := authJSONRequest(t, ac, "POST", "/logout", fmt.Sprintf(`{"refresh_token":%q}`, failedLogoutRefresh))
	if failedLogout.Code != 500 {
		t.Fatalf("expected logout to fail when audit insert fails, got %d: %s", failedLogout.Code, failedLogout.Body.String())
	}
	var activeLogoutTokens, activeLogoutSessions int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("token_hash = ? AND revoked = false", failedLogoutHash).Count(&activeLogoutTokens).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Table(tableSchema + ".sessions").Where("refresh_token_hash = ? AND revoked = false", failedLogoutHash).Count(&activeLogoutSessions).Error; err != nil {
		t.Fatal(err)
	}
	if activeLogoutTokens != 1 || activeLogoutSessions != 1 {
		t.Fatalf("logout audit failure did not roll back revocation: active tokens=%d sessions=%d", activeLogoutTokens, activeLogoutSessions)
	}
	if err := admin.Exec("DROP TRIGGER reject_auth_audit ON " + tableSchema + ".audit_logs").Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("DROP FUNCTION " + privateSchema + ".reject_auth_audit()").Error; err != nil {
		t.Fatal(err)
	}

	// A failed refresh audit insert must roll back both token rotation writes.
	if err := admin.Exec("CREATE FUNCTION " + privateSchema + ".reject_auth_audit() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RAISE EXCEPTION ''audit sink unavailable''; END'").Error; err != nil {
		t.Fatalf("create refresh audit failure trigger function: %v", err)
	}
	if err := admin.Exec("CREATE TRIGGER reject_auth_audit BEFORE INSERT ON " + tableSchema + ".audit_logs FOR EACH ROW EXECUTE FUNCTION " + privateSchema + ".reject_auth_audit()").Error; err != nil {
		t.Fatalf("install refresh audit failure trigger: %v", err)
	}
	failedRefresh := authJSONRequest(t, ac, "POST", "/refresh", fmt.Sprintf(`{"refresh_token":%q}`, rawRefresh))
	if failedRefresh.Code != 500 {
		t.Fatalf("expected refresh to fail when audit insert fails, got %d: %s", failedRefresh.Code, failedRefresh.Body.String())
	}
	var unrotated int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("token_hash = ? AND revoked = false", auth.HashRefreshToken(rawRefresh)).Count(&unrotated).Error; err != nil {
		t.Fatal(err)
	}
	if unrotated != 1 {
		t.Fatal("refresh audit failure left the original token revoked")
	}
	if err := admin.Exec("DROP TRIGGER reject_auth_audit ON " + tableSchema + ".audit_logs").Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("DROP FUNCTION " + privateSchema + ".reject_auth_audit()").Error; err != nil {
		t.Fatal(err)
	}

	_, rotatedRefresh := decodeAuthTokens(t, authJSONRequest(t, ac, "POST", "/refresh", fmt.Sprintf(`{"refresh_token":%q}`, rawRefresh)))
	if rotatedRefresh == rawRefresh {
		t.Fatal("expected PostgreSQL handler to rotate the refresh token")
	}
	var priorToken struct {
		FamilyID uuid.UUID
		Revoked bool
	}
	if err := admin.Table(tableSchema + ".refresh_tokens").Select("family_id", "revoked").Where("token_hash = ?", auth.HashRefreshToken(rawRefresh)).Scan(&priorToken).Error; err != nil {
		t.Fatal(err)
	}
	if !priorToken.Revoked {
		t.Fatalf("rotation did not persist the old token as revoked (family %s)", priorToken.FamilyID)
	}
	var refreshAuditCount int64
	if err := admin.Table(tableSchema + ".audit_logs").Where("action = ? AND resource_id = ?", "auth.refresh", userID).Count(&refreshAuditCount).Error; err != nil {
		t.Fatal(err)
	}
	if refreshAuditCount != 1 {
		t.Fatalf("expected exactly one committed refresh audit event, got %d", refreshAuditCount)
	}
	// A failed security audit insert must roll back family revocation as well.
	if err := admin.Exec("CREATE FUNCTION " + privateSchema + ".reject_auth_audit() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RAISE EXCEPTION ''audit sink unavailable''; END'").Error; err != nil {
		t.Fatalf("recreate audit failure trigger function: %v", err)
	}
	if err := admin.Exec("CREATE TRIGGER reject_auth_audit BEFORE INSERT ON " + tableSchema + ".audit_logs FOR EACH ROW EXECUTE FUNCTION " + privateSchema + ".reject_auth_audit()").Error; err != nil {
		t.Fatalf("reinstall audit failure trigger: %v", err)
	}
	failedReuse := authJSONRequest(t, ac, "POST", "/refresh", fmt.Sprintf(`{"refresh_token":%q}`, rawRefresh))
	if failedReuse.Code != 500 {
		t.Fatalf("expected reuse response to fail when audit insert fails, got %d: %s", failedReuse.Code, failedReuse.Body.String())
	}
	var activeAfterAuditFailure int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("family_id = ? AND revoked = false", priorToken.FamilyID).Count(&activeAfterAuditFailure).Error; err != nil {
		t.Fatal(err)
	}
	if activeAfterAuditFailure != 1 {
		t.Fatalf("audit failure should roll back family revocation, found %d active tokens", activeAfterAuditFailure)
	}
	if err := admin.Exec("DROP TRIGGER reject_auth_audit ON " + tableSchema + ".audit_logs").Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("DROP FUNCTION " + privateSchema + ".reject_auth_audit()").Error; err != nil {
		t.Fatal(err)
	}

	reused := authJSONRequest(t, ac, "POST", "/refresh", fmt.Sprintf(`{"refresh_token":%q}`, rawRefresh))
	if reused.Code != 401 || !strings.Contains(reused.Body.String(), "already used") {
		t.Fatalf("expected PostgreSQL handler to detect reused token, got %d: %s", reused.Code, reused.Body.String())
	}

	var activeTokens int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("family_id = ? AND revoked = false", priorToken.FamilyID).Count(&activeTokens).Error; err != nil {
		t.Fatal(err)
	}
	if activeTokens != 0 {
		t.Fatalf("expected reuse detection to revoke family %s, found %d active tokens (response %s)", priorToken.FamilyID, activeTokens, reused.Body.String())
	}
	var auditCount int64
	if err := admin.Table(tableSchema + ".audit_logs").Where("action = ? AND resource_id = ?", "security.refresh_token_reuse", userID).Count(&auditCount).Error; err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("expected one refresh-token reuse audit event, found %d", auditCount)
	}

	// A pending school must not receive a session even though the credential
	// lookup succeeds and the password is correct.
	if err := admin.Table(tableSchema + ".schools").Where("id = ?", schoolID).Update("status", models.SchoolStatusPending).Error; err != nil {
		t.Fatal(err)
	}
	pending := authJSONRequest(t, ac, "POST", "/login", loginBody)
	if pending.Code != 401 {
		t.Fatalf("expected pending-school PostgreSQL login to fail, got %d: %s", pending.Code, pending.Body.String())
	}
}
