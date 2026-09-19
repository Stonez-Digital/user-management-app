package middleware

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/auth"
    "github.com/onoja217/users-management-app/internal/authz"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestAuthMiddlewareUsesCurrentDatabaseRole(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.User{}); err != nil { t.Fatal(err) }

    if err := auth.ConfigureSecret("this-is-a-test-secret-with-at-least-32-chars"); err != nil { t.Fatal(err) }
    user := models.User{ID: uuid.New(), Name: "Test", Email: "test@example.com", Role: authz.RoleStudent, Active: true}
    if err := db.Create(&user).Error; err != nil { t.Fatal(err) }

    token, err := auth.GenerateToken(user.ID.String(), user.Role)
    if err != nil { t.Fatal(err) }

    if err := db.Model(&user).Update("role", authz.RoleSchoolAdmin).Error; err != nil { t.Fatal(err) }

    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(AuthMiddleware(db), RequirePermission(authz.PermissionUsersRead))
    r.GET("/admin/users", func(c *gin.Context) { c.Status(http.StatusNoContent) })

    req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != http.StatusNoContent {
        t.Fatalf("expected current database role to authorize request, got %d: %s", w.Code, w.Body.String())
    }
}

func TestRequirePermissionDeniesUnauthorizedRole(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(func(c *gin.Context) {
        c.Set(RoleKey, authz.RoleStudent)
        c.Next()
    }, RequirePermission(authz.PermissionUsersRead))
    r.GET("/admin/users", func(c *gin.Context) { c.Status(http.StatusNoContent) })

    req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != http.StatusForbidden {
        t.Fatalf("expected forbidden response, got %d", w.Code)
    }
}
