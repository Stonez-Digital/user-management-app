package controller

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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
	tableSchema, privateSchema := "auth_handler_"+suffix, "auth_private_"+suffix
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
		"CREATE TABLE " + tableSchema + ".password_reset_tokens (id uuid PRIMARY KEY, user_id uuid NOT NULL, token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL, used boolean NOT NULL, created_at timestamptz)",
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
		script = strings.ReplaceAll(script, "stonez_internal", privateSchema)
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

	// Logout-all must revoke every session and refresh token for this user,
	// while leaving another user's sessions untouched.
	logoutAllUserID, otherUserID := uuid.New(), uuid.New()
	logoutAllFamilyA, logoutAllFamilyB, otherFamily := uuid.New(), uuid.New(), uuid.New()
	logoutAllHashA := auth.HashRefreshToken("postgres-logout-all-A-" + suffix)
	logoutAllHashB := auth.HashRefreshToken("postgres-logout-all-B-" + suffix)
	otherHash := auth.HashRefreshToken("postgres-logout-all-other-" + suffix)
	for _, item := range []struct{ family, user uuid.UUID; hash string }{
		{logoutAllFamilyA, logoutAllUserID, logoutAllHashA},
		{logoutAllFamilyB, logoutAllUserID, logoutAllHashB},
		{otherFamily, otherUserID, otherHash},
	} {
		if err := admin.Exec("INSERT INTO " + tableSchema + ".refresh_tokens (id,family_id,user_id,token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)", uuid.New(), item.family, item.user, item.hash, time.Now().Add(time.Hour)).Error; err != nil {
			t.Fatal(err)
		}
		if err := admin.Exec("INSERT INTO " + tableSchema + ".sessions (id,user_id,family_id,refresh_token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)", uuid.New(), item.user, item.family, item.hash, time.Now().Add(time.Hour)).Error; err != nil {
			t.Fatal(err)
		}
	}
	logoutAllRouter := gin.New()
	logoutAllRouter.POST("/logout-all", func(c *gin.Context) {
		c.Set("user_id", logoutAllUserID.String())
		ac.LogoutAll(c)
	})
	logoutAllReq := httptest.NewRequest("POST", "/logout-all", nil)
	logoutAllRec := httptest.NewRecorder()
	logoutAllRouter.ServeHTTP(logoutAllRec, logoutAllReq)
	if logoutAllRec.Code != 200 {
		t.Fatalf("restricted PostgreSQL logout-all failed: %d %s", logoutAllRec.Code, logoutAllRec.Body.String())
	}
	var logoutAllRevokedTokens, logoutAllRevokedSessions, logoutAllAudit, otherStillActive int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("user_id = ? AND revoked = true AND token_hash IN ?", logoutAllUserID, []string{logoutAllHashA, logoutAllHashB}).Count(&logoutAllRevokedTokens).Error; err != nil { t.Fatal(err) }
	if err := admin.Table(tableSchema + ".sessions").Where("user_id = ? AND revoked = true AND refresh_token_hash IN ?", logoutAllUserID, []string{logoutAllHashA, logoutAllHashB}).Count(&logoutAllRevokedSessions).Error; err != nil { t.Fatal(err) }
	if err := admin.Table(tableSchema + ".audit_logs").Where("action = ? AND resource_id = ?", "auth.logout_all", logoutAllUserID).Count(&logoutAllAudit).Error; err != nil { t.Fatal(err) }
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("user_id = ? AND revoked = false", otherUserID).Count(&otherStillActive).Error; err != nil { t.Fatal(err) }
	if logoutAllRevokedTokens != 2 || logoutAllRevokedSessions != 2 || logoutAllAudit != 1 || otherStillActive != 1 {
		t.Fatalf("logout-all scope/audit incorrect: tokens=%d sessions=%d audits=%d other-user-active=%d", logoutAllRevokedTokens, logoutAllRevokedSessions, logoutAllAudit, otherStillActive)
	}

	// Individual session revocation must revoke only the selected session and
	// its paired refresh token, never a sibling session or another user's row.
	sessionOwnerUserID := uuid.New()
	selectedSessionID, siblingSessionID := uuid.New(), uuid.New()
	selectedFamilyID, siblingFamilyID := uuid.New(), uuid.New()
	selectedHash := auth.HashRefreshToken("postgres-selected-session-" + suffix)
	siblingHash := auth.HashRefreshToken("postgres-sibling-session-" + suffix)
	for _, item := range []struct{ id, family uuid.UUID; hash string }{
		{selectedSessionID, selectedFamilyID, selectedHash},
		{siblingSessionID, siblingFamilyID, siblingHash},
	} {
		if err := admin.Exec("INSERT INTO " + tableSchema + ".refresh_tokens (id,family_id,user_id,token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)", uuid.New(), item.family, sessionOwnerUserID, item.hash, time.Now().Add(time.Hour)).Error; err != nil { t.Fatal(err) }
		if err := admin.Exec("INSERT INTO " + tableSchema + ".sessions (id,user_id,family_id,refresh_token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)", item.id, sessionOwnerUserID, item.family, item.hash, time.Now().Add(time.Hour)).Error; err != nil { t.Fatal(err) }
	}
	revokeSessionRouter := gin.New()
	revokeSessionRouter.POST("/sessions/:id/revoke", func(c *gin.Context) {
		c.Set("user_id", sessionOwnerUserID.String())
		ac.RevokeSession(c)
	})
	revokeSessionReq := httptest.NewRequest("POST", "/sessions/"+selectedSessionID.String()+"/revoke", nil)
	revokeSessionRec := httptest.NewRecorder()
	revokeSessionRouter.ServeHTTP(revokeSessionRec, revokeSessionReq)
	if revokeSessionRec.Code != 200 {
		t.Fatalf("restricted PostgreSQL individual session revocation failed: %d %s", revokeSessionRec.Code, revokeSessionRec.Body.String())
	}
	var selectedSessionRevoked, selectedTokenRevoked, siblingSessionActive, siblingTokenActive, sessionRevokeAudit int64
	if err := admin.Table(tableSchema + ".sessions").Where("id = ? AND revoked = true", selectedSessionID).Count(&selectedSessionRevoked).Error; err != nil { t.Fatal(err) }
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("token_hash = ? AND revoked = true", selectedHash).Count(&selectedTokenRevoked).Error; err != nil { t.Fatal(err) }
	if err := admin.Table(tableSchema + ".sessions").Where("id = ? AND revoked = false", siblingSessionID).Count(&siblingSessionActive).Error; err != nil { t.Fatal(err) }
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("token_hash = ? AND revoked = false", siblingHash).Count(&siblingTokenActive).Error; err != nil { t.Fatal(err) }
	if err := admin.Table(tableSchema + ".audit_logs").Where("action = ? AND resource_id = ?", "auth.session_revoke", selectedSessionID).Count(&sessionRevokeAudit).Error; err != nil { t.Fatal(err) }
	if selectedSessionRevoked != 1 || selectedTokenRevoked != 1 || siblingSessionActive != 1 || siblingTokenActive != 1 || sessionRevokeAudit != 1 {
		t.Fatalf("individual session revocation scope/audit incorrect: selected-session=%d selected-token=%d sibling-session-active=%d sibling-token-active=%d audits=%d", selectedSessionRevoked, selectedTokenRevoked, siblingSessionActive, siblingTokenActive, sessionRevokeAudit)
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

	// Password recovery must work through the restricted runtime identity.
	mailer := &passwordResetMailerStub{}
	ac.Mailer = mailer
	forgotRouter := gin.New()
	forgotRouter.POST("/forgot-password", ac.ForgotPassword)
	forgotReq := httptest.NewRequest("POST", "/forgot-password", strings.NewReader(fmt.Sprintf(`{"email":%q}`, email)))
	forgotReq.Header.Set("Content-Type", "application/json")
	forgotRec := httptest.NewRecorder()
	forgotRouter.ServeHTTP(forgotRec, forgotReq)
	if forgotRec.Code != 200 || mailer.calls != 1 || mailer.token == "" {
		t.Fatalf("restricted PostgreSQL forgot-password failed: status=%d body=%s mailer=%+v", forgotRec.Code, forgotRec.Body.String(), mailer)
	}
	var storedReset models.PasswordResetToken
	if err := admin.Table(tableSchema + ".password_reset_tokens").Where("user_id = ?", userID).Scan(&storedReset).Error; err != nil {
		t.Fatal(err)
	}
	if storedReset.TokenHash != auth.HashResetToken(mailer.token) {
		t.Fatal("PostgreSQL reset storage did not retain only the reset-token hash")
	}

	// Reset consumes the token, changes the password, revokes sessions/tokens,
	// and records its audit event in one transaction.
	resetFamily := uuid.New()
	resetRefresh := "postgres-reset-refresh-" + suffix
	resetHash := auth.HashRefreshToken(resetRefresh)
	if err := admin.Exec("INSERT INTO " + tableSchema + ".refresh_tokens (id,family_id,user_id,token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)", uuid.New(), resetFamily, userID, resetHash, time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("INSERT INTO " + tableSchema + ".sessions (id,user_id,family_id,refresh_token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)", uuid.New(), userID, resetFamily, resetHash, time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	resetRouter := gin.New()
	resetRouter.POST("/reset-password", ac.ResetPassword)
	resetReq := httptest.NewRequest("POST", "/reset-password", strings.NewReader(fmt.Sprintf(`{"token":%q,"new_password":"new-reset-password123"}`, mailer.token)))
	resetReq.Header.Set("Content-Type", "application/json")
	resetRec := httptest.NewRecorder()
	resetRouter.ServeHTTP(resetRec, resetReq)
	if resetRec.Code != 200 {
		t.Fatalf("restricted PostgreSQL password reset failed: %d %s", resetRec.Code, resetRec.Body.String())
	}
	var updatedPassword string
	if err := admin.Table(tableSchema + ".users").Select("password_hash").Where("id = ?", userID).Scan(&updatedPassword).Error; err != nil {
		t.Fatal(err)
	}
	if !auth.CheckPassword(updatedPassword, "new-reset-password123") {
		t.Fatal("password reset did not persist the new password hash")
	}
	var resetRevokedTokens, resetRevokedSessions, resetAudit int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("family_id = ? AND revoked = true", resetFamily).Count(&resetRevokedTokens).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Table(tableSchema + ".sessions").Where("family_id = ? AND revoked = true", resetFamily).Count(&resetRevokedSessions).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Table(tableSchema + ".audit_logs").Where("action = ? AND resource_id = ?", "security.password_reset", userID).Count(&resetAudit).Error; err != nil {
		t.Fatal(err)
	}
	if resetRevokedTokens != 1 || resetRevokedSessions != 1 || resetAudit != 1 {
		t.Fatalf("password reset transaction incomplete: revoked tokens=%d sessions=%d audit=%d", resetRevokedTokens, resetRevokedSessions, resetAudit)
	}
	replayReq := httptest.NewRequest("POST", "/reset-password", strings.NewReader(fmt.Sprintf(`{"token":%q,"new_password":"another-reset-password123"}`, mailer.token)))
	replayReq.Header.Set("Content-Type", "application/json")
	replayRec := httptest.NewRecorder()
	resetRouter.ServeHTTP(replayRec, replayReq)
	if replayRec.Code != 401 {
		t.Fatalf("expected PostgreSQL reset-token replay to fail, got %d: %s", replayRec.Code, replayRec.Body.String())
	}

	// Audit failure must roll back the password update, reset-token consumption,
	// and session revocations together.
	failedResetRaw := "postgres-reset-audit-failure-" + suffix
	failedResetID := uuid.New()
	if err := admin.Exec("INSERT INTO " + tableSchema + ".password_reset_tokens (id,user_id,token_hash,expires_at,used) VALUES (?,?,?,?,false)", failedResetID, userID, auth.HashResetToken(failedResetRaw), time.Now().Add(15*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	failedResetFamily := uuid.New()
	failedResetHash := auth.HashRefreshToken("postgres-reset-audit-refresh-" + suffix)
	if err := admin.Exec("INSERT INTO " + tableSchema + ".refresh_tokens (id,family_id,user_id,token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)", uuid.New(), failedResetFamily, userID, failedResetHash, time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("INSERT INTO " + tableSchema + ".sessions (id,user_id,family_id,refresh_token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)", uuid.New(), userID, failedResetFamily, failedResetHash, time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("CREATE FUNCTION " + privateSchema + ".reject_auth_audit() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RAISE EXCEPTION ''audit sink unavailable''; END'").Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("CREATE TRIGGER reject_auth_audit BEFORE INSERT ON " + tableSchema + ".audit_logs FOR EACH ROW EXECUTE FUNCTION " + privateSchema + ".reject_auth_audit()").Error; err != nil {
		t.Fatal(err)
	}
	failedResetReq := httptest.NewRequest("POST", "/reset-password", strings.NewReader(fmt.Sprintf(`{"token":%q,"new_password":"must-not-commit123"}`, failedResetRaw)))
	failedResetReq.Header.Set("Content-Type", "application/json")
	failedResetRec := httptest.NewRecorder()
	resetRouter.ServeHTTP(failedResetRec, failedResetReq)
	if failedResetRec.Code != 500 {
		t.Fatalf("expected password reset audit failure to return 500, got %d: %s", failedResetRec.Code, failedResetRec.Body.String())
	}
	var failedResetStillUnused, failedResetStillActive, failedSessionStillActive int64
	if err := admin.Table(tableSchema + ".password_reset_tokens").Where("id = ? AND used = false", failedResetID).Count(&failedResetStillUnused).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("family_id = ? AND revoked = false", failedResetFamily).Count(&failedResetStillActive).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Table(tableSchema + ".sessions").Where("family_id = ? AND revoked = false", failedResetFamily).Count(&failedSessionStillActive).Error; err != nil {
		t.Fatal(err)
	}
	if failedResetStillUnused != 1 || failedResetStillActive != 1 || failedSessionStillActive != 1 {
		t.Fatalf("audit failure did not roll back reset: unused=%d active tokens=%d active sessions=%d", failedResetStillUnused, failedResetStillActive, failedSessionStillActive)
	}
	if err := admin.Exec("DROP TRIGGER reject_auth_audit ON " + tableSchema + ".audit_logs").Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("DROP FUNCTION " + privateSchema + ".reject_auth_audit()").Error; err != nil {
		t.Fatal(err)
	}

	// Password change uses the restricted credential lookup and writer function.
	changeFamily := uuid.New()
	changeHash := auth.HashRefreshToken("postgres-change-password-refresh-" + suffix)
	if err := admin.Exec("INSERT INTO " + tableSchema + ".refresh_tokens (id,family_id,user_id,token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)", uuid.New(), changeFamily, userID, changeHash, time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("INSERT INTO " + tableSchema + ".sessions (id,user_id,family_id,refresh_token_hash,expires_at,revoked) VALUES (?,?,?,?,?,false)", uuid.New(), userID, changeFamily, changeHash, time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	changeRouter := gin.New()
	changeRouter.POST("/change-password", func(c *gin.Context) {
		c.Set("user_id", userID.String())
		ac.ChangePassword(c)
	})
	changeReq := httptest.NewRequest("POST", "/change-password", strings.NewReader(`{"current_password":"new-reset-password123","new_password":"changed-password123"}`))
	changeReq.Header.Set("Content-Type", "application/json")
	changeRec := httptest.NewRecorder()
	changeRouter.ServeHTTP(changeRec, changeReq)
	if changeRec.Code != 200 {
		t.Fatalf("restricted PostgreSQL password change failed: %d %s", changeRec.Code, changeRec.Body.String())
	}
	if err := admin.Table(tableSchema + ".users").Select("password_hash").Where("id = ?", userID).Scan(&updatedPassword).Error; err != nil {
		t.Fatal(err)
	}
	if !auth.CheckPassword(updatedPassword, "changed-password123") {
		t.Fatal("password change did not persist the new password hash")
	}
	var changeRevokedTokens, changeRevokedSessions, changeAudit int64
	if err := admin.Table(tableSchema + ".refresh_tokens").Where("family_id = ? AND revoked = true", changeFamily).Count(&changeRevokedTokens).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Table(tableSchema + ".sessions").Where("family_id = ? AND revoked = true", changeFamily).Count(&changeRevokedSessions).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Table(tableSchema + ".audit_logs").Where("action = ? AND resource_id = ?", "security.password_change", userID).Count(&changeAudit).Error; err != nil {
		t.Fatal(err)
	}
	if changeRevokedTokens != 1 || changeRevokedSessions != 1 || changeAudit != 1 {
		t.Fatalf("password change transaction incomplete: revoked tokens=%d sessions=%d audit=%d", changeRevokedTokens, changeRevokedSessions, changeAudit)
	}

	// Two concurrent reset requests must consume the same token exactly once.
	concurrentResetRaw := "postgres-concurrent-reset-" + suffix
	concurrentResetID := uuid.New()
	if err := admin.Exec("INSERT INTO " + tableSchema + ".password_reset_tokens (id,user_id,token_hash,expires_at,used,created_at) VALUES (?,?,?,?,false,clock_timestamp())", concurrentResetID, userID, auth.HashResetToken(concurrentResetRaw), time.Now().Add(15*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	concurrentRouter := gin.New()
	concurrentRouter.POST("/reset-password", ac.ResetPassword)
	startConcurrentReset := make(chan struct{})
	concurrentResults := make(chan int, 2)
	var concurrentWG sync.WaitGroup
	for _, password := range []string{"concurrent-reset-password-A123", "concurrent-reset-password-B123"} {
		concurrentWG.Add(1)
		go func(newPassword string) {
			defer concurrentWG.Done()
			<-startConcurrentReset
			req := httptest.NewRequest("POST", "/reset-password", strings.NewReader(fmt.Sprintf(`{"token":%q,"new_password":%q}`, concurrentResetRaw, newPassword)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			concurrentRouter.ServeHTTP(rec, req)
			concurrentResults <- rec.Code
		}(password)
	}
	close(startConcurrentReset)
	concurrentWG.Wait()
	close(concurrentResults)
	concurrentSuccesses, concurrentRejections := 0, 0
	for status := range concurrentResults {
		switch status {
		case 200:
			concurrentSuccesses++
		case 401:
			concurrentRejections++
		default:
			t.Fatalf("unexpected concurrent reset response status %d", status)
		}
	}
	if concurrentSuccesses != 1 || concurrentRejections != 1 {
		t.Fatalf("expected one successful reset and one rejected replay, got success=%d rejected=%d", concurrentSuccesses, concurrentRejections)
	}
	var concurrentTokenUsed int64
	if err := admin.Table(tableSchema + ".password_reset_tokens").Where("id = ? AND used = true", concurrentResetID).Count(&concurrentTokenUsed).Error; err != nil {
		t.Fatal(err)
	}
	if concurrentTokenUsed != 1 {
		t.Fatal("concurrent reset token was not consumed exactly once")
	}
}
