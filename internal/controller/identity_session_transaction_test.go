package controller

import (
	"gorm.io/gorm"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/auth"
	"github.com/onoja217/users-management-app/internal/authz"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"github.com/onoja217/users-management-app/internal/service"
)

func identityRequest(t *testing.T, db *gorm.DB, userID uuid.UUID, method, path, body string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(middleware.UserIDKey, userID.String()); c.Next() })
	router.Handle(method, path, middleware.IdentityTransaction(db), handler)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" { req.Header.Set("Content-Type", "application/json") }
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestMeIdentityTransactionSupportsSchoolAndPlatformUsers(t *testing.T) {
	db := authControllerTestDB(t)
	school := models.School{ID: uuid.New(), Name: "Identity School", Code: "IDENTITY", Slug: "identity-school", Status: models.SchoolStatusActive}
	if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
	schoolUser := models.User{ID: uuid.New(), Name: "School User", Email: "school-identity@example.test", Role: authz.RoleSchoolAdmin, Active: true, SchoolID: &school.ID}
	platformUser := models.User{ID: uuid.New(), Name: "Platform User", Email: "platform-identity@example.test", Role: authz.RoleSuperAdmin, Active: true}
	otherUser := models.User{ID: uuid.New(), Name: "Other User", Email: "other-identity@example.test", Role: authz.RoleStudent, Active: true, SchoolID: &school.ID}
	for _, user := range []*models.User{&schoolUser, &platformUser, &otherUser} {
		if err := db.Create(user).Error; err != nil { t.Fatal(err) }
	}
	ctrl := NewUserController(service.NewUserService(repository.NewUserRepository(db), db))

	rec := identityRequest(t, db, schoolUser.ID, http.MethodGet, "/me", "", ctrl.GetMe)
	if rec.Code != http.StatusOK { t.Fatalf("school user /me failed: %d %s", rec.Code, rec.Body.String()) }
	var profile map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil { t.Fatal(err) }
	if profile["school_name"] != school.Name { t.Fatalf("expected linked school projection, got %#v", profile) }

	rec = identityRequest(t, db, platformUser.ID, http.MethodGet, "/me", "", ctrl.GetMe)
	if rec.Code != http.StatusOK { t.Fatalf("platform user without school /me failed: %d %s", rec.Code, rec.Body.String()) }
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil { t.Fatal(err) }
	if profile["school_id"] != nil && profile["school_id"] != "" { t.Fatalf("platform profile unexpectedly has school: %#v", profile["school_id"]) }

	rec = identityRequest(t, db, schoolUser.ID, http.MethodPut, "/me", `{"name":"Updated Own Name"}`, ctrl.UpdateMe)
	if rec.Code != http.StatusOK { t.Fatalf("own profile update failed: %d %s", rec.Code, rec.Body.String()) }
	var unchanged models.User
	if err := db.First(&unchanged, "id = ?", otherUser.ID).Error; err != nil { t.Fatal(err) }
	if unchanged.Name != otherUser.Name { t.Fatalf("profile update changed another user: %q", unchanged.Name) }
	var updated models.User
	if err := db.First(&updated, "id = ?", schoolUser.ID).Error; err != nil { t.Fatal(err) }
	if updated.Name != "Updated Own Name" { t.Fatalf("own profile was not updated: %q", updated.Name) }
}

func TestSessionEndpointsAreUserScopedAndAtomic(t *testing.T) {
	db := authControllerTestDB(t)
	userA, userB := uuid.New(), uuid.New()
	familyA, familyB := uuid.New(), uuid.New()
	hashA, hashB := auth.HashRefreshToken("refresh-token-a"), auth.HashRefreshToken("refresh-token-b")
	now := time.Now().Add(time.Hour)
	sessionA := models.Session{ID: uuid.New(), UserID: userA, FamilyID: familyA, RefreshTokenHash: hashA, ExpiresAt: now}
	sessionB := models.Session{ID: uuid.New(), UserID: userB, FamilyID: familyB, RefreshTokenHash: hashB, ExpiresAt: now}
	for _, session := range []models.Session{sessionA, sessionB} {
		if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	}
	for _, token := range []models.RefreshToken{
		{ID: uuid.New(), UserID: userA, FamilyID: familyA, TokenHash: hashA, ExpiresAt: now},
		{ID: uuid.New(), UserID: userB, FamilyID: familyB, TokenHash: hashB, ExpiresAt: now},
	} {
		if err := db.Create(&token).Error; err != nil { t.Fatal(err) }
	}
	ac := NewAuthController(db)

	rec := identityRequest(t, db, userA, http.MethodGet, "/sessions", "", ac.GetSessions)
	if rec.Code != http.StatusOK { t.Fatalf("session list failed: %d %s", rec.Code, rec.Body.String()) }
	var listed []models.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil { t.Fatal(err) }
	if len(listed) != 1 || listed[0].ID != sessionA.ID { t.Fatalf("session list crossed user boundary: %+v", listed) }

	rec = identityRequest(t, db, userA, http.MethodDelete, "/sessions/"+sessionB.ID.String(), "", ac.RevokeSession)
	if rec.Code != http.StatusNotFound { t.Fatalf("cross-user session revoke should be hidden: %d %s", rec.Code, rec.Body.String()) }
	var untouched models.Session
	if err := db.First(&untouched, "id = ?", sessionB.ID).Error; err != nil { t.Fatal(err) }
	if untouched.Revoked { t.Fatal("cross-user session was revoked") }

	rec = identityRequest(t, db, userA, http.MethodDelete, "/sessions/"+sessionA.ID.String(), "", ac.RevokeSession)
	if rec.Code != http.StatusOK { t.Fatalf("own session revoke failed: %d %s", rec.Code, rec.Body.String()) }
	var revokedSession models.Session
	if err := db.First(&revokedSession, "id = ?", sessionA.ID).Error; err != nil { t.Fatal(err) }
	var revokedToken models.RefreshToken
	if err := db.First(&revokedToken, "token_hash = ?", hashA).Error; err != nil { t.Fatal(err) }
	if !revokedSession.Revoked || !revokedToken.Revoked { t.Fatal("session and refresh token must be revoked together") }
	if err := db.First(&untouched, "id = ?", sessionB.ID).Error; err != nil { t.Fatal(err) }
	if untouched.Revoked { t.Fatal("another user's session changed") }
}

func TestLogoutAllRevokesOnlyAuthenticatedUsersSessionsAndTokens(t *testing.T) {
	db := authControllerTestDB(t)
	userA, userB := uuid.New(), uuid.New()
	now := time.Now().Add(time.Hour)
	for _, userID := range []uuid.UUID{userA, userB} {
		family := uuid.New()
		hash := auth.HashRefreshToken(userID.String())
		if err := db.Create(&models.Session{ID: uuid.New(), UserID: userID, FamilyID: family, RefreshTokenHash: hash, ExpiresAt: now}).Error; err != nil { t.Fatal(err) }
		if err := db.Create(&models.RefreshToken{ID: uuid.New(), UserID: userID, FamilyID: family, TokenHash: hash, ExpiresAt: now}).Error; err != nil { t.Fatal(err) }
	}
	ac := NewAuthController(db)
	rec := identityRequest(t, db, userA, http.MethodPost, "/logout-all", "", ac.LogoutAll)
	if rec.Code != http.StatusOK { t.Fatalf("logout-all failed: %d %s", rec.Code, rec.Body.String()) }
	var activeA, activeB int64
	if err := db.Model(&models.Session{}).Where("user_id = ? AND revoked = false", userA).Count(&activeA).Error; err != nil { t.Fatal(err) }
	if err := db.Model(&models.RefreshToken{}).Where("user_id = ? AND revoked = false", userA).Count(&activeA).Error; err != nil { t.Fatal(err) }
	if err := db.Model(&models.Session{}).Where("user_id = ? AND revoked = false", userB).Count(&activeB).Error; err != nil { t.Fatal(err) }
	if activeA != 0 || activeB != 1 { t.Fatalf("logout-all scope wrong: userA active=%d userB active=%d", activeA, activeB) }
}

func TestIdentityHandlersFailClosedWithoutIdentityTransaction(t *testing.T) {
	db := authControllerTestDB(t)
	userController := NewUserController(nil)
	authController := NewAuthController(db)
	tests := []struct{name string; handler gin.HandlerFunc; method string; path string; body string}{
		{"get me", userController.GetMe, http.MethodGet, "/me", ""},
		{"update me", userController.UpdateMe, http.MethodPut, "/me", `{"name":"No Context"}`},
		{"list sessions", authController.GetSessions, http.MethodGet, "/sessions", ""},
		{"revoke session", authController.RevokeSession, http.MethodDelete, "/sessions/"+uuid.NewString(), ""},
		{"logout all", authController.LogoutAll, http.MethodPost, "/logout-all", ""},
		{"change password", authController.ChangePassword, http.MethodPost, "/change-password", `{"current_password":"old-password123","new_password":"new-password123"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set(middleware.UserIDKey, uuid.NewString()); c.Next() })
			router.Handle(tc.method, tc.path, tc.handler)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" { req.Header.Set("Content-Type", "application/json") }
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusInternalServerError { t.Fatalf("expected missing identity transaction to fail closed, got %d: %s", rec.Code, rec.Body.String()) }
			if !strings.Contains(rec.Body.String(), "identity_transaction_required") { t.Fatalf("expected identity_transaction_required, got %s", rec.Body.String()) }
		})
	}
}

func TestSessionRevocationRollsBackWhenRefreshTokenRevocationFails(t *testing.T) {
	db := authControllerTestDB(t)
	userID, sessionID, familyID := uuid.New(), uuid.New(), uuid.New()
	hash := auth.HashRefreshToken("atomic-revoke-refresh")
	now := time.Now().Add(time.Hour)
	if err := db.Create(&models.Session{ID: sessionID, UserID: userID, FamilyID: familyID, RefreshTokenHash: hash, ExpiresAt: now}).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&models.RefreshToken{ID: uuid.New(), UserID: userID, FamilyID: familyID, TokenHash: hash, ExpiresAt: now}).Error; err != nil { t.Fatal(err) }
	if err := db.Exec(`CREATE TRIGGER fail_refresh_revoke BEFORE UPDATE OF revoked ON refresh_tokens BEGIN SELECT RAISE(ABORT, 'forced refresh revocation failure'); END`).Error; err != nil { t.Fatal(err) }
	ac := NewAuthController(db)
	rec := identityRequest(t, db, userID, http.MethodDelete, "/sessions/"+sessionID.String(), "", ac.RevokeSession)
	if rec.Code != http.StatusInternalServerError { t.Fatalf("expected atomic revocation failure, got %d: %s", rec.Code, rec.Body.String()) }
	var session models.Session
	if err := db.First(&session, "id = ?", sessionID).Error; err != nil { t.Fatal(err) }
	if session.Revoked { t.Fatal("session revocation must roll back when refresh-token revocation fails") }
	var token models.RefreshToken
	if err := db.First(&token, "token_hash = ?", hash).Error; err != nil { t.Fatal(err) }
	if token.Revoked { t.Fatal("refresh-token revocation unexpectedly committed") }
}
