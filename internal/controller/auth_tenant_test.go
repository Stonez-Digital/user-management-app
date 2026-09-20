package controller

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/onoja217/users-management-app/internal/authz"
    "github.com/onoja217/users-management-app/internal/middleware"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func authTenantTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.RoleChangeAudit{}); err != nil { t.Fatal(err) }
    return db
}

func tenantContextRouter(handler gin.HandlerFunc, schoolID string, userID string, method string, path string, body string) *httptest.ResponseRecorder {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Handle(method, path, func(c *gin.Context) {
        c.Set(middleware.SchoolIDKey, schoolID)
        c.Set(middleware.UserIDKey, userID)
        if id := strings.TrimPrefix(path, "/admin/users/"); id != path {
            id = strings.TrimSuffix(strings.TrimSuffix(id, "/deactivate"), "/role")
            c.Params = gin.Params{{Key: "id", Value: id}}
        }
        handler(c)
    })
    req := httptest.NewRequest(method, path, strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    r.ServeHTTP(rec, req)
    return rec
}

func TestAdminUserMutationsRejectCrossSchoolTargets(t *testing.T) {
    db := authTenantTestDB(t)
    schoolA := models.School{Name: "School A", Code: "A", Status: models.SchoolStatusActive}
    schoolB := models.School{Name: "School B", Code: "B", Status: models.SchoolStatusActive}
    if err := db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&schoolB).Error; err != nil { t.Fatal(err) }

    actor := models.User{Name: "Admin A", Email: "admin-a@example.com", Role: authz.RoleSchoolAdmin, Active: true, SchoolID: &schoolA.ID}
    target := models.User{Name: "Target B", Email: "target-b@example.com", Role: authz.RoleStudent, Active: true, SchoolID: &schoolB.ID}
    if err := db.Create(&actor).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&target).Error; err != nil { t.Fatal(err) }

    ac := NewAuthController(db)

    rec := tenantContextRouter(ac.DeactivateUser, schoolA.ID.String(), actor.ID.String(), http.MethodPost, "/admin/users/"+target.ID.String()+"/deactivate", "")
    if rec.Code != http.StatusNotFound { t.Fatalf("expected cross-school deactivate to return 404, got %d: %s", rec.Code, rec.Body.String()) }

    rec = tenantContextRouter(ac.AssignRole, schoolA.ID.String(), actor.ID.String(), http.MethodPut, "/admin/users/"+target.ID.String()+"/role", `{"role":"teacher"}`)
    if rec.Code != http.StatusNotFound { t.Fatalf("expected cross-school role assignment to return 404, got %d: %s", rec.Code, rec.Body.String()) }

    var stored models.User
    if err := db.First(&stored, "id = ?", target.ID).Error; err != nil { t.Fatal(err) }
    if !stored.Active || stored.Role != authz.RoleStudent {
        t.Fatal("cross-school administrator mutation changed the target user")
    }
}
