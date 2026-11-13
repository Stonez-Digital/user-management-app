package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/onoja217/users-management-app/internal/repository"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAttendanceEndpointsFailClosedWithoutTenantTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:attendance_no_tenant_tx?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	ctrl := NewAttendanceController(service.NewAttendanceService(repository.NewAttendanceRepository(db), db))
	gin.SetMode(gin.TestMode)
	routes := []struct { method, path string; handler gin.HandlerFunc }{
		{http.MethodGet, "/admin/attendance", ctrl.List},
		{http.MethodGet, "/admin/attendance/:id", ctrl.Get},
		{http.MethodPost, "/admin/attendance", ctrl.Create},
		{http.MethodPut, "/admin/attendance/:id", ctrl.Update},
		{http.MethodDelete, "/admin/attendance/:id", ctrl.Delete},
		{http.MethodGet, "/teacher/attendance", ctrl.TeacherList},
		{http.MethodPost, "/teacher/attendance", ctrl.TeacherCreate},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			r := gin.New()
			r.Handle(route.method, route.path, route.handler)
			path := strings.ReplaceAll(route.path, ":id", "11111111-1111-4111-8111-111111111111")
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
