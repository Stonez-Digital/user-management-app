package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/auth"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthMiddlewareRejectsRevokedSessionAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := auth.ConfigureSecret("test-auth-secret-for-middleware-tests-32chars"); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Session{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{ID: uuid.New(), Name: "Session Test", Email: "session-test@example.com", Role: "student", Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	session := models.Session{ID: uuid.New(), UserID: user.ID, RefreshTokenHash: "test-refresh-hash", ExpiresAt: time.Now().Add(time.Hour), Revoked: true}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateTokenForSession(user.ID.String(), user.Role, session.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/protected", AuthMiddleware(db), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked session token to return 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddlewareAcceptsActiveSessionAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := auth.ConfigureSecret("test-auth-secret-for-middleware-tests-32chars"); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Session{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{ID: uuid.New(), Name: "Active Session Test", Email: "active-session-test@example.com", Role: "student", Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	session := models.Session{ID: uuid.New(), UserID: user.ID, RefreshTokenHash: "active-test-refresh-hash", ExpiresAt: time.Now().Add(time.Hour), Revoked: false}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	token, err := auth.GenerateTokenForSession(user.ID.String(), user.Role, session.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/protected", AuthMiddleware(db), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected active session token to pass, got %d: %s", rec.Code, rec.Body.String())
	}
}


func TestAuthMiddlewareLoadsSchoolContextInsideAuthenticatedTransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if err := auth.ConfigureSecret("test-auth-secret-for-middleware-tests-32chars"); err != nil { t.Fatal(err) }
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.User{}, &models.Session{}); err != nil { t.Fatal(err) }
	schoolID := uuid.New()
	user := models.User{ID: uuid.New(), Name: "School Context", Email: "school-context@example.com", Role: "school_admin", Active: true, SchoolID: &schoolID}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	session := models.Session{ID: uuid.New(), UserID: user.ID, RefreshTokenHash: "school-context-refresh-hash", ExpiresAt: time.Now().Add(time.Hour)}
	if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	token, err := auth.GenerateTokenForSession(user.ID.String(), user.Role, session.ID.String())
	if err != nil { t.Fatal(err) }

	router := gin.New()
	router.GET("/protected", AuthMiddleware(db), RequireSchoolContext(nil), func(c *gin.Context) {
		if c.GetString(UserIDKey) != user.ID.String() { c.Status(http.StatusInternalServerError); return }
		if c.GetString(SchoolIDKey) != schoolID.String() { c.Status(http.StatusInternalServerError); return }
		if c.GetString(RoleKey) != user.Role { c.Status(http.StatusInternalServerError); return }
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent { t.Fatalf("expected school context to be propagated from authenticated user, got %d: %s", rec.Code, rec.Body.String()) }
}

func TestRequireSchoolContextFailsClosedWithoutAuthenticatedSchool(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/school", RequireSchoolContext(nil), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/school", nil))
	if rec.Code != http.StatusForbidden { t.Fatalf("expected missing school context to be forbidden, got %d", rec.Code) }
}
