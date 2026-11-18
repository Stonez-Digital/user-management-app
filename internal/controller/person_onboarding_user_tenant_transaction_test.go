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

func TestPersonOnboardingAndUserEndpointsFailClosedWithoutTenantTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:person_user_no_tenant_tx?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	users := NewUserController(service.NewUserService(repository.NewUserRepository(db), db))
	onboarding := NewPersonOnboardingController(service.NewPersonOnboardingService(db))
	gin.SetMode(gin.TestMode)

	routes := []struct {
		method, path string
		handler      gin.HandlerFunc
	}{
		{http.MethodPost, "/onboarding/people", onboarding.Create},
		{http.MethodPost, "/users", users.CreateUser},
		{http.MethodGet, "/users", users.GetUsers},
		{http.MethodGet, "/users/:id", users.GetUser},
		{http.MethodDelete, "/users/:id", users.DeleteUser},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(middleware.SchoolIDKey, uuid.NewString())
				c.Set(middleware.UserIDKey, uuid.NewString())
				c.Next()
			})
			r.Handle(route.method, route.path, route.handler)

			path := strings.ReplaceAll(route.path, ":id", uuid.NewString())
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(route.method, path, nil))

			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("expected missing tenant transaction to fail closed with 500, got %d: %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "tenant_transaction_required") {
				t.Fatalf("expected tenant transaction error, got %s", recorder.Body.String())
			}
		})
	}
}
