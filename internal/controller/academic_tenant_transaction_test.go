package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func academicTenantDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:academic_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.AcademicSession{}, &models.Term{}); err != nil { t.Fatal(err) }
	return db
}

func academicTenantRequest(db *gorm.DB, schoolID, userID uuid.UUID, withTransaction bool, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.SchoolIDKey, schoolID.String())
		c.Set(middleware.UserIDKey, userID.String())
		c.Next()
	})
	if withTransaction { r.Use(middleware.TenantTransaction(db)) }
	r.GET("/academic-sessions", handler)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/academic-sessions", nil))
	return recorder
}

func TestAcademicControllerUsesRequestScopedTransaction(t *testing.T) {
	db := academicTenantDB(t)
	schoolA, schoolB, userID := uuid.New(), uuid.New(), uuid.New()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC)
	for _, item := range []struct{ school uuid.UUID; name string }{{schoolA, "School A session"}, {schoolB, "School B session"}} {
		if err := db.Create(&models.AcademicSession{SchoolID: item.school, Name: item.name, StartDate: start, EndDate: end, Status: models.AcademicStatusPlanned}).Error; err != nil { t.Fatal(err) }
	}
	svc := service.NewAcademicService(repository.NewAcademicSessionRepository(db), repository.NewTermRepository(db), db)
	ctrl := NewAcademicController(svc)
	recorder := academicTenantRequest(db, schoolA, userID, true, ctrl.ListSessions)
	if recorder.Code != http.StatusOK { t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String()) }
	var sessions []models.AcademicSession
	if err := json.Unmarshal(recorder.Body.Bytes(), &sessions); err != nil { t.Fatal(err) }
	if len(sessions) != 1 || sessions[0].Name != "School A session" {
		t.Fatalf("school A must only receive its own academic sessions, got %+v", sessions)
	}
}

func TestAcademicControllerFailsClosedWithoutTenantTransaction(t *testing.T) {
	db := academicTenantDB(t)
	svc := service.NewAcademicService(repository.NewAcademicSessionRepository(db), repository.NewTermRepository(db), db)
	ctrl := NewAcademicController(svc)
	recorder := academicTenantRequest(db, uuid.New(), uuid.New(), false, ctrl.ListSessions)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected missing tenant transaction to fail closed with 500, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
