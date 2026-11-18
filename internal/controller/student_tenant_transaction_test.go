package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func studentTenantControllerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:student_tenant_controller_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.Student{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func studentTenantRequest(db *gorm.DB, schoolID, userID uuid.UUID, withTransaction bool, path string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.SchoolIDKey, schoolID.String())
		c.Set(middleware.UserIDKey, userID.String())
		c.Next()
	})
	if withTransaction {
		r.Use(middleware.TenantTransaction(db))
	}
	r.GET(path, handler)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func TestStudentControllerUsesTenantTransactionForSchoolScopedReads(t *testing.T) {
	db := studentTenantControllerDB(t)
	schoolA := models.School{Name: "Tenant A", Code: "TENANT-A", Status: models.SchoolStatusActive}
	schoolB := models.School{Name: "Tenant B", Code: "TENANT-B", Status: models.SchoolStatusActive}
	if err := db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&schoolB).Error; err != nil { t.Fatal(err) }

	userA := models.User{Name: "Student A", Email: "student-a@tenant-test.example", Role: "student", Active: true, SchoolID: &schoolA.ID}
	userB := models.User{Name: "Student B", Email: "student-b@tenant-test.example", Role: "student", Active: true, SchoolID: &schoolB.ID}
	if err := db.Create(&userA).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&userB).Error; err != nil { t.Fatal(err) }

	studentA := models.Student{SchoolID: schoolA.ID, UserID: userA.ID, AdmissionNumber: "A-001"}
	studentB := models.Student{SchoolID: schoolB.ID, UserID: userB.ID, AdmissionNumber: "B-001"}
	if err := db.Create(&studentA).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&studentB).Error; err != nil { t.Fatal(err) }

	ctrl := NewStudentController(service.NewStudentService(repository.NewStudentRepository(db), db))
	recorder := studentTenantRequest(db, schoolA.ID, userA.ID, true, "/students", ctrl.List)
	if recorder.Code != http.StatusOK { t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String()) }
	var visible []struct { ID uuid.UUID `json:"id"` }
	if err := json.Unmarshal(recorder.Body.Bytes(), &visible); err != nil { t.Fatal(err) }
	if len(visible) != 1 || visible[0].ID != studentA.ID {
		t.Fatalf("school A must only receive its own student through the request transaction, got %+v", visible)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.SchoolIDKey, schoolA.ID.String())
		c.Set(middleware.UserIDKey, userA.ID.String())
		c.Next()
	})
	r.Use(middleware.TenantTransaction(db))
	r.GET("/students/:id", ctrl.Get)
	recorder = httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/students/"+studentB.ID.String(), nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("school A must not resolve school B's student, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestStudentControllerFailsClosedWithoutTenantTransaction(t *testing.T) {
	db := studentTenantControllerDB(t)
	schoolID, userID := uuid.New(), uuid.New()
	ctrl := NewStudentController(service.NewStudentService(repository.NewStudentRepository(db), db))
	recorder := studentTenantRequest(db, schoolID, userID, false, "/students", ctrl.List)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected missing tenant transaction to fail closed with 500, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
