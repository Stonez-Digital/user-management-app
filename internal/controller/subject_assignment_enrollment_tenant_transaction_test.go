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

func TestSubjectAssignmentAndEnrollmentEndpointsFailClosedWithoutTenantTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:subject_assignment_enrollment_no_tx?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	subjects := NewSubjectController(service.NewSubjectService(repository.NewSubjectRepository(db), db))
	classSubjects := NewClassSubjectController(service.NewClassSubjectService(repository.NewClassSubjectRepository(db), db))
	assignments := NewTeacherAssignmentController(service.NewTeacherAssignmentService(repository.NewTeacherAssignmentRepository(db), db))
	enrollments := NewEnrollmentController(service.NewEnrollmentService(repository.NewEnrollmentRepository(db), db))

	gin.SetMode(gin.TestMode)
	routes := []struct {
		method string
		path string
		handler gin.HandlerFunc
	}{
		{http.MethodGet, "/admin/subjects", subjects.List},
		{http.MethodGet, "/admin/subjects/:id", subjects.Get},
		{http.MethodPost, "/admin/subjects", subjects.Create},
		{http.MethodPut, "/admin/subjects/:id", subjects.Update},
		{http.MethodDelete, "/admin/subjects/:id", subjects.Delete},
		{http.MethodGet, "/admin/class-subjects", classSubjects.List},
		{http.MethodGet, "/admin/class-subjects/:id", classSubjects.Get},
		{http.MethodPost, "/admin/class-subjects", classSubjects.Create},
		{http.MethodPut, "/admin/class-subjects/:id", classSubjects.Update},
		{http.MethodDelete, "/admin/class-subjects/:id", classSubjects.Delete},
		{http.MethodGet, "/admin/teacher-assignments", assignments.List},
		{http.MethodGet, "/admin/teacher-assignments/coverage", assignments.Coverage},
		{http.MethodGet, "/admin/teacher-assignments/:id", assignments.Get},
		{http.MethodPost, "/admin/teacher-assignments", assignments.Create},
		{http.MethodPut, "/admin/teacher-assignments/:id", assignments.Update},
		{http.MethodDelete, "/admin/teacher-assignments/:id", assignments.Delete},
		{http.MethodGet, "/admin/enrollments", enrollments.List},
		{http.MethodGet, "/admin/enrollments/student/:studentId", enrollments.History},
		{http.MethodGet, "/admin/enrollments/:id", enrollments.Get},
		{http.MethodPost, "/admin/enrollments", enrollments.Create},
		{http.MethodPost, "/admin/enrollments/:id/place", enrollments.Place},
		{http.MethodPut, "/admin/enrollments/:id", enrollments.Update},
		{http.MethodDelete, "/admin/enrollments/:id", enrollments.Delete},
		{http.MethodGet, "/teacher/enrollments", enrollments.TeacherList},
		{http.MethodGet, "/teacher/assignments", assignments.TeacherList},
	}

	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			r := gin.New()
			r.Handle(route.method, route.path, route.handler)
			path := strings.ReplaceAll(route.path, ":id", "11111111-1111-4111-8111-111111111111")
			path = strings.ReplaceAll(path, ":studentId", "11111111-1111-4111-8111-111111111111")
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
