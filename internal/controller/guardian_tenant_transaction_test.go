package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/repository"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGuardianLinkMutationFailsClosedWithoutTenantTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:guardian_controller_no_tx?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewGuardianService(repository.NewGuardianRepository(db), db)
	ctrl := NewGuardianController(svc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/guardian-links", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, uuid.NewString())
		c.Set(middleware.SchoolIDKey, uuid.NewString())
		ctrl.CreateLink(c)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/guardian-links", strings.NewReader(`{"guardian_user_id":"`+uuid.NewString()+`","student_id":"`+uuid.NewString()+`","relationship":"parent"}`))
	request.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected missing tenant transaction to fail closed with 500, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "tenant_transaction_required") {
		t.Fatalf("expected tenant transaction error, got %s", recorder.Body.String())
	}
}


func TestGuardianReadFailsClosedWithoutTenantTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:guardian_read_no_tx?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ctrl := NewGuardianController(service.NewGuardianService(repository.NewGuardianRepository(db), db))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/children", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, uuid.NewString())
		c.Set(middleware.SchoolIDKey, uuid.NewString())
		ctrl.Children(c)
	})
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/children", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected guardian read without tenant transaction to fail closed with 500, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "tenant_transaction_required") {
		t.Fatalf("expected tenant transaction error, got %s", recorder.Body.String())
	}
}
