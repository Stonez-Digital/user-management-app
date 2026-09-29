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
    if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.RoleChangeAudit{}, &models.TeacherProfile{}, &models.ParentProfile{}); err != nil { t.Fatal(err) }
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

func TestAssignRoleRejectsRoleProfileMismatch(t *testing.T) {
    db := authTenantTestDB(t)
    school := models.School{Name: "School", Code: "S", Status: models.SchoolStatusActive}
    if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
    actor := models.User{Name: "Admin", Email: "admin@example.com", Role: authz.RoleSchoolAdmin, Active: true, SchoolID: &school.ID}
    parent := models.User{Name: "Parent", Email: "parent@example.com", Role: authz.RoleParent, Active: true, SchoolID: &school.ID}
    teacher := models.User{Name: "Teacher", Email: "teacher@example.com", Role: authz.RoleTeacher, Active: true, SchoolID: &school.ID}
    student := models.User{Name: "Student", Email: "student@example.com", Role: authz.RoleStudent, Active: true, SchoolID: &school.ID}
    for _, u := range []*models.User{&actor, &parent, &teacher, &student} { if err := db.Create(u).Error; err != nil { t.Fatal(err) } }
    if err := db.Create(&models.ParentProfile{SchoolID: school.ID, UserID: parent.ID, ParentIdentifier: "P-1"}).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&models.TeacherProfile{SchoolID: school.ID, UserID: teacher.ID, StaffID: "T-1"}).Error; err != nil { t.Fatal(err) }
    ac := NewAuthController(db)
    rec := tenantContextRouter(ac.AssignRole, school.ID.String(), actor.ID.String(), http.MethodPut, "/admin/users/"+parent.ID.String()+"/role", `{"role":"student"}`)
    if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "role_profile_mismatch") { t.Fatalf("expected parent profile mismatch conflict, got %d: %s", rec.Code, rec.Body.String()) }
    rec = tenantContextRouter(ac.AssignRole, school.ID.String(), actor.ID.String(), http.MethodPut, "/admin/users/"+teacher.ID.String()+"/role", `{"role":"staff"}`)
    if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "role_profile_mismatch") { t.Fatalf("expected teacher profile mismatch conflict, got %d: %s", rec.Code, rec.Body.String()) }
    rec = tenantContextRouter(ac.AssignRole, school.ID.String(), actor.ID.String(), http.MethodPut, "/admin/users/"+student.ID.String()+"/role", `{"role":"teacher"}`)
    if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "role_profile_mismatch") { t.Fatalf("expected missing teacher profile conflict, got %d: %s", rec.Code, rec.Body.String()) }
    var stored models.User
    if err := db.First(&stored, "id = ?", parent.ID).Error; err != nil { t.Fatal(err) }
    if stored.Role != authz.RoleParent { t.Fatalf("parent role changed unexpectedly: %s", stored.Role) }
    if err := db.First(&stored, "id = ?", teacher.ID).Error; err != nil { t.Fatal(err) }
    if stored.Role != authz.RoleTeacher { t.Fatalf("teacher role changed unexpectedly: %s", stored.Role) }
    if err := db.First(&stored, "id = ?", student.ID).Error; err != nil { t.Fatal(err) }
    if stored.Role != authz.RoleStudent { t.Fatalf("student role changed unexpectedly: %s", stored.Role) }
}
