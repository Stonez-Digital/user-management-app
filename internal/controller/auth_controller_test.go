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
    if err := db.AutoMigrate(&models.User{}, &models.RefreshToken{}, &models.Session{}, &models.PasswordResetToken{}, &models.AuditLog{}); err != nil { t.Fatal(err) }
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
    var reuseAudit models.AuditLog
    if err := db.Where("action = ? AND resource_id = ?", "security.refresh_token_reuse", user.ID).First(&reuseAudit).Error; err != nil { t.Fatalf("expected refresh-token reuse audit event: %v", err) }
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

func TestResetPasswordConsumesHashedTokenAndRevokesSessions(t *testing.T) {
    db := authControllerTestDB(t)
    hash, err := auth.HashPassword("old-password123")
    if err != nil { t.Fatal(err) }
    user := models.User{Name: "Reset Test", Email: "reset-"+strings.ToLower(t.Name())+"@example.com", PasswordHash: hash, Role: authz.RoleStudent, Active: true}
    if err := db.Create(&user).Error; err != nil { t.Fatal(err) }

    rawToken := strings.Repeat("reset-token-", 4)
    familyID := uuid.New()
    if err := db.Create(&models.RefreshToken{
        ID: uuid.New(), UserID: user.ID, FamilyID: familyID,
        TokenHash: auth.HashRefreshToken("refresh-reset-test"),
        ExpiresAt: time.Now().Add(time.Hour), Revoked: false,
    }).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&models.Session{
        ID: uuid.New(), UserID: user.ID, FamilyID: familyID,
        RefreshTokenHash: auth.HashRefreshToken("refresh-reset-test"),
        ExpiresAt: time.Now().Add(time.Hour), Revoked: false,
    }).Error; err != nil { t.Fatal(err) }
    reset := models.PasswordResetToken{
        ID: uuid.New(), UserID: user.ID, TokenHash: auth.HashResetToken(rawToken),
        ExpiresAt: time.Now().Add(15 * time.Minute), Used: false,
    }
    if err := db.Create(&reset).Error; err != nil { t.Fatal(err) }

    router := gin.New()
    router.POST("/reset-password", NewAuthController(db).ResetPassword)
    req := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(
        "{\"token\":\""+rawToken+"\",\"new_password\":\"new-password123\"}",
    ))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)
    if rec.Code != http.StatusOK { t.Fatalf("expected reset 200, got %d: %s", rec.Code, rec.Body.String()) }

    var stored models.PasswordResetToken
    if err := db.First(&stored, "id = ?", reset.ID).Error; err != nil { t.Fatal(err) }
    if stored.Used != true { t.Fatal("expected reset token to be marked used") }
    if stored.TokenHash != auth.HashResetToken(rawToken) { t.Fatal("expected reset token to remain hashed") }

    var session models.Session
    if err := db.Where("user_id = ?", user.ID).First(&session).Error; err != nil { t.Fatal(err) }
    if !session.Revoked { t.Fatal("expected password reset to revoke sessions") }

    reuseReq := httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(
        "{\"token\":\""+rawToken+"\",\"new_password\":\"another-password123\"}",
    ))
    reuseReq.Header.Set("Content-Type", "application/json")
    reuseRec := httptest.NewRecorder()
    router.ServeHTTP(reuseRec, reuseReq)
    if reuseRec.Code != http.StatusUnauthorized { t.Fatalf("expected reused reset token to be rejected, got %d: %s", reuseRec.Code, reuseRec.Body.String()) }
}


func TestRefreshRejectsInactiveUser(t *testing.T) {
    db := authControllerTestDB(t)
    user := models.User{Name: "Inactive Refresh", Email: "inactive-refresh-"+strings.ToLower(t.Name())+"@example.com", Role: authz.RoleStudent, Active: false}
    if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
    if err := db.Model(&user).Update("active", false).Error; err != nil { t.Fatal(err) }
    refresh := strings.Repeat("inactive-refresh-token-", 4)
    if err := db.Create(&models.RefreshToken{ID: uuid.New(), UserID: user.ID, FamilyID: uuid.New(), TokenHash: auth.HashRefreshToken(refresh), ExpiresAt: time.Now().Add(time.Hour), Revoked: false}).Error; err != nil { t.Fatal(err) }
    rec := authJSONRequest(t, NewAuthController(db), http.MethodPost, "/refresh", "{\"refresh_token\":\""+refresh+"\"}")
    if rec.Code != http.StatusUnauthorized { t.Fatalf("expected inactive user refresh to be rejected, got %d: %s", rec.Code, rec.Body.String()) }
}

func TestRefreshRejectsMalformedOrUnknownToken(t *testing.T) {
    db := authControllerTestDB(t)
    ac := NewAuthController(db)
    for _, token := range []string{"not-a-jwt", "short", strings.Repeat("x", 32)} {
        rec := authJSONRequest(t, ac, http.MethodPost, "/refresh", "{\"refresh_token\":\""+token+"\"}")
        if rec.Code != http.StatusUnauthorized { t.Fatalf("expected token %q to be rejected, got %d: %s", token, rec.Code, rec.Body.String()) }
    }
}

func TestPasswordChangeRevokesAllSessionsAndRefreshTokens(t *testing.T) {
    db := authControllerTestDB(t)
    hash, err := auth.HashPassword("current-password123")
    if err != nil { t.Fatal(err) }
    user := models.User{Name: "Password Change", Email: "password-change-"+strings.ToLower(t.Name())+"@example.com", PasswordHash: hash, Role: authz.RoleStudent, Active: true}
    if err := db.Create(&user).Error; err != nil { t.Fatal(err) }

    for i := 0; i < 2; i++ {
        familyID := uuid.New()
        refresh := strings.Repeat("change-password-refresh-"+string(rune('a'+i)), 3)
        if err := db.Create(&models.RefreshToken{ID: uuid.New(), UserID: user.ID, FamilyID: familyID, TokenHash: auth.HashRefreshToken(refresh), ExpiresAt: time.Now().Add(time.Hour), Revoked: false}).Error; err != nil { t.Fatal(err) }
        if err := db.Create(&models.Session{ID: uuid.New(), UserID: user.ID, FamilyID: familyID, RefreshTokenHash: auth.HashRefreshToken(refresh), ExpiresAt: time.Now().Add(time.Hour), Revoked: false}).Error; err != nil { t.Fatal(err) }
    }

    ac := NewAuthController(db)
    router := gin.New()
    router.POST("/change-password", func(c *gin.Context) {
        c.Set("user_id", user.ID.String())
        ac.ChangePassword(c)
    })
    req := httptest.NewRequest(http.MethodPost, "/change-password", strings.NewReader("{\"current_password\":\"current-password123\",\"new_password\":\"new-password123\"}"))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    router.ServeHTTP(rec, req)
    if rec.Code != http.StatusOK { t.Fatalf("expected password change 200, got %d: %s", rec.Code, rec.Body.String()) }

    var tokens []models.RefreshToken
    if err := db.Where("user_id = ?", user.ID).Find(&tokens).Error; err != nil { t.Fatal(err) }
    for _, token := range tokens { if !token.Revoked { t.Fatalf("expected refresh token %s to be revoked", token.ID) } }
    var sessions []models.Session
    if err := db.Where("user_id = ?", user.ID).Find(&sessions).Error; err != nil { t.Fatal(err) }
    for _, session := range sessions { if !session.Revoked { t.Fatalf("expected session %s to be revoked", session.ID) } }
}
