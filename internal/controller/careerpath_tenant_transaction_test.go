package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCareerPathAttemptHandlersFailClosedWithoutTenantTransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:careerpath_tenant_transaction?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	controller := NewCareerPathController(service.NewCareerPathService(db))

	tests := []struct {
		name string
		path string
		handler gin.HandlerFunc
		method string
	}{
		{name: "create or resume", method: http.MethodPost, path: "/attempts", handler: controller.CreateOrResume},
		{name: "get attempt", method: http.MethodGet, path: "/attempts/" + uuid.NewString(), handler: controller.Get},
		{name: "save responses", method: http.MethodPut, path: "/attempts/" + uuid.NewString() + "/responses", handler: controller.SaveResponses},
		{name: "submit attempt", method: http.MethodPost, path: "/attempts/" + uuid.NewString() + "/submit", handler: controller.Submit},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(middleware.UserIDKey, uuid.NewString())
				c.Set(middleware.SchoolIDKey, uuid.NewString())
				c.Next()
			})
			router.Handle(test.method, test.path, test.handler)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("expected missing tenant transaction to fail closed with 500, got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
