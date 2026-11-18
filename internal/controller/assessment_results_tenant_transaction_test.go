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

func TestAssessmentAndResultEndpointsFailClosedWithoutTenantTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:assessment_results_no_tenant_tx?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	assessment := NewAssessmentController(service.NewAssessmentService(repository.NewAssessmentRepository(db), db))
	result := NewAssessmentResultController(service.NewAssessmentResultService(repository.NewAssessmentResultRepository(db), db))
	gin.SetMode(gin.TestMode)
	routes := []struct { method, path string; handler gin.HandlerFunc }{
		{http.MethodGet, "/admin/assessments", assessment.List},
		{http.MethodGet, "/admin/assessments/:id", assessment.Get},
		{http.MethodPost, "/admin/assessments", assessment.Create},
		{http.MethodPut, "/admin/assessments/:id", assessment.Update},
		{http.MethodDelete, "/admin/assessments/:id", assessment.Delete},
		{http.MethodGet, "/teacher/assessments", assessment.TeacherList},
		{http.MethodPost, "/teacher/assessments", assessment.TeacherCreate},
		{http.MethodGet, "/admin/assessment-results", result.List},
		{http.MethodGet, "/admin/assessment-results/:id", result.Get},
		{http.MethodPost, "/admin/assessment-results", result.Create},
		{http.MethodPut, "/admin/assessment-results/:id", result.Update},
		{http.MethodDelete, "/admin/assessment-results/:id", result.Delete},
		{http.MethodGet, "/admin/report-cards/:enrollmentId", result.ReportCard},
		{http.MethodGet, "/teacher/results", result.TeacherList},
		{http.MethodPost, "/teacher/results", result.TeacherCreate},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			r := gin.New()
			r.Handle(route.method, route.path, route.handler)
			path := strings.ReplaceAll(route.path, ":id", "11111111-1111-4111-8111-111111111111")
			path = strings.ReplaceAll(path, ":enrollmentId", "11111111-1111-4111-8111-111111111111")
			recorder := httptest.NewRecorder()
			r.ServeHTTP(recorder, httptest.NewRequest(route.method, path+"?academic_session_id=11111111-1111-4111-8111-111111111111&term_id=11111111-1111-4111-8111-111111111111", nil))
			if recorder.Code != http.StatusInternalServerError {
				t.Fatalf("expected missing tenant transaction to fail closed with 500, got %d: %s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), "tenant_transaction_required") {
				t.Fatalf("expected tenant transaction error, got %s", recorder.Body.String())
			}
		})
	}
}
