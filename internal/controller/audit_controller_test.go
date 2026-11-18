package controller

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/middleware"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestAuditListIsSchoolScoped(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file:audit_controller_test?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.AuditLog{}); err != nil { t.Fatal(err) }

    schoolA := models.School{ID: uuid.New(), Name: "School A", Code: "AUDIT-A"}
    schoolB := models.School{ID: uuid.New(), Name: "School B", Code: "AUDIT-B"}
    if err := db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&schoolB).Error; err != nil { t.Fatal(err) }

    userA := models.User{ID: uuid.New(), SchoolID: &schoolA.ID, Name: "Admin A", Email: "audit-a@example.com", Active: true}
    userB := models.User{ID: uuid.New(), SchoolID: &schoolB.ID, Name: "Admin B", Email: "audit-b@example.com", Active: true}
    if err := db.Create(&userA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&userB).Error; err != nil { t.Fatal(err) }

    logs := []models.AuditLog{
        {ID: uuid.New(), SchoolID: &schoolA.ID, ActorID: &userA.ID, Action: "school_a.action", Resource: "user"},
        {ID: uuid.New(), SchoolID: &schoolB.ID, ActorID: &userB.ID, Action: "school_b.action", Resource: "user"},
    }
    if err := db.Create(&logs).Error; err != nil { t.Fatal(err) }

    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(func(c *gin.Context) {
        c.Set(middleware.SchoolIDKey, schoolA.ID.String())
        c.Next()
    })
    r.GET("/admin/audit-logs", NewAuditController(db).List)

    req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs", nil)
    rec := httptest.NewRecorder()
    r.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK { t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String()) }
    body := rec.Body.String()
    if !contains(body, "school_a.action") { t.Fatalf("expected School A audit entry, got %s", body) }
    if contains(body, "school_b.action") { t.Fatalf("cross-school audit entry leaked: %s", body) }
}

func contains(s, sub string) bool {
    return len(sub) == 0 || stringIndex(s, sub) >= 0
}

func stringIndex(s, sub string) int {
    for i := 0; i+len(sub) <= len(s); i++ {
        if s[i:i+len(sub)] == sub { return i }
    }
    return -1
}
