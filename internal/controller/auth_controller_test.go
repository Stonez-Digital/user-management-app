package controller

import (
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
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func authControllerTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    if err := auth.ConfigureSecret("test-auth-secret-for-controller-tests-32chars"); err != nil { t.Fatal(err) }
    db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.User{}, &models.RefreshToken{}, &models.Session{}, &models.AuditLog{}); err != nil { t.Fatal(err) }
    return db
}

func authJSONRequest(t *testing.T, ac *AuthController, method, path, body string) *httptest.ResponseRecorder {
    t.Helper()
    gin.SetMode(gin.TestMode)
    router := gin.New()
    switch path {
    case "/login": router.POST(path, ac.Login)
    case "/refresh": router.POST(path, ac.Refresh)
    case "/logout": router.POST(path, ac.Logout)
    default: t.Fatalf("unsupported test path %s", path)
    }
    req := httptest.NewRequest(method, path, strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)
    return rec
}

func decodeAuthTokens(t *testing.T, rec *httptest.ResponseRecorder) (string, string) {
    t.Helper()
    if rec.Code != http.StatusOK { t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String()) }
    var payload struct {
        AccessToken string `json:"access_token"`
        RefreshToken string `json:"refresh_token"`
    }
    if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil { t.Fatal(err) }
    if payload.AccessToken == "" || payload.RefreshToken == "" { t.Fatal("expected access and refresh tokens") }
    return payload.AccessToken, payload.RefreshToken
}

func TestRefreshRotatesTokenAndRejectsOldToken(t *testing.T) {
    db := authControllerTestDB(t)
    hash, err := auth.HashPassword("password123")
    if err != nil { t.Fatal(err) }
    user := models.User{Name: "Refresh Test", Email: "refresh-" + strings.ToLower(t.Name()) + "@example.com", PasswordHash: hash, Role: authz.RoleStudent, Active: true}
    if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
    ac := NewAuthController(db)
    _, refresh := decodeAuthTokens(t, authJSONRequest(t, ac, http.MethodPost, "/login", "{\"email\":\""+user.Email+"\",\"password\":\"password123\"}"))
    rec := authJSONRequest(t, ac, http.MethodPost, "/refresh", "{\"refresh_token\":\""+refresh+"\"}")
    _, rotated := decodeAuthTokens(t, rec)
    if rotated == refresh { t.Fatal("expected refresh token rotation") }
    reused := authJSONRequest(t, ac, http.MethodPost, "/refresh", "{\"refresh_token\":\""+refresh+"\"}")
    if reused.Code != http.StatusUnauthorized { t.Fatalf("expected reused token to be rejected, got %d: %s", reused.Code, reused.Body.String()) }
    var tokens []models.RefreshToken
    if err := db.Where("user_id = ?", user.ID).Find(&tokens).Error; err != nil { t.Fatal(err) }
    for _, token := range tokens { if !token.Revoked { t.Fatalf("expected token %s to be revoked after reuse detection", token.ID) } }
    var sessions []models.Session
    if err := db.Where("user_id = ?", user.ID).Find(&sessions).Error; err != nil { t.Fatal(err) }
    for _, session := range sessions { if !session.Revoked { t.Fatalf("expected session %s to be revoked after reuse detection", session.ID) } }
}

func TestLogoutRevokesRefreshTokenAndSession(t *testing.T) {
    db := authControllerTestDB(t)
    hash, err := auth.HashPassword("password123")
    if err != nil { t.Fatal(err) }
    user := models.User{Name: "Logout Test", Email: "logout-" + strings.ToLower(t.Name()) + "@example.com", PasswordHash: hash, Role: authz.RoleStudent, Active: true}
    if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
    ac := NewAuthController(db)
    _, refresh := decodeAuthTokens(t, authJSONRequest(t, ac, http.MethodPost, "/login", "{\"email\":\""+user.Email+"\",\"password\":\"password123\"}"))
    rec := authJSONRequest(t, ac, http.MethodPost, "/logout", "{\"refresh_token\":\""+refresh+"\"}")
    if rec.Code != http.StatusOK { t.Fatalf("expected logout 200, got %d: %s", rec.Code, rec.Body.String()) }
    var token models.RefreshToken
    if err := db.Where("token_hash = ?", auth.HashRefreshToken(refresh)).First(&token).Error; err != nil { t.Fatal(err) }
    if !token.Revoked { t.Fatal("expected refresh token to be revoked") }
    var session models.Session
    if err := db.Where("refresh_token_hash = ?", auth.HashRefreshToken(refresh)).First(&session).Error; err != nil { t.Fatal(err) }
    if !session.Revoked { t.Fatal("expected session to be revoked") }
    reuse := authJSONRequest(t, ac, http.MethodPost, "/refresh", "{\"refresh_token\":\""+refresh+"\"}")
    if reuse.Code != http.StatusUnauthorized { t.Fatalf("expected revoked token to be rejected, got %d", reuse.Code) }
}

func TestRefreshRejectsExpiredToken(t *testing.T) {
    db := authControllerTestDB(t)
    user := models.User{Name: "Expired Test", Email: "expired-" + t.Name() + "@example.com", Role: authz.RoleStudent, Active: true}
    if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
    refresh := strings.Repeat("expired-refresh-token-", 4)
    if err := db.Create(&models.RefreshToken{ID: uuid.New(), UserID: user.ID, FamilyID: uuid.New(), TokenHash: auth.HashRefreshToken(refresh), ExpiresAt: time.Now().Add(-time.Minute), Revoked: false}).Error; err != nil { t.Fatal(err) }
    rec := authJSONRequest(t, NewAuthController(db), http.MethodPost, "/refresh", "{\"refresh_token\":\""+refresh+"\"}")
    if rec.Code != http.StatusUnauthorized { t.Fatalf("expected expired token to be rejected, got %d: %s", rec.Code, rec.Body.String()) }
}
