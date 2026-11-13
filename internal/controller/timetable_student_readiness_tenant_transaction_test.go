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

func TestTimetableStudentPortalAndReadinessFailClosedWithoutTenantTransaction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:timetable_student_readiness_no_tx?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }

	timetable := NewTimetableController(service.NewTimetableService(repository.NewTimetableRepository(db), db))
	student := NewStudentPortalController(service.NewStudentPortalService(db))
	readiness := NewAcademicReadinessController(service.NewAcademicReadinessService(db))

	gin.SetMode(gin.TestMode)
	routes := []struct {
		method string
		path string
		handler gin.HandlerFunc
	}{
		{http.MethodGet, "/admin/timetable", timetable.List},
		{http.MethodGet, "/admin/timetable/:id", timetable.Get},
		{http.MethodPost, "/admin/timetable", timetable.Create},
		{http.MethodPut, "/admin/timetable/:id", timetable.Update},
		{http.MethodDelete, "/admin/timetable/:id", timetable.Delete},
		{http.MethodGet, "/teacher/timetable", timetable.TeacherView},
		{http.MethodGet, "/admin/school/readiness", readiness.Get},
		{http.MethodGet, "/student/profile", student.Profile},
		{http.MethodGet, "/student/enrollment", student.Enrollment},
		{http.MethodGet, "/student/terms", student.Terms},
		{http.MethodGet, "/student/attendance", student.Attendance},
		{http.MethodGet, "/student/timetable", student.Timetable},
		{http.MethodGet, "/student/report-cards/:termId", student.ReportCard},
		{http.MethodGet, "/student/invoices", student.Invoices},
		{http.MethodGet, "/student/payments", student.Payments},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			r := gin.New()
			r.Handle(route.method, route.path, route.handler)
			path := strings.ReplaceAll(route.path, ":id", "11111111-1111-4111-8111-111111111111")
			path = strings.ReplaceAll(path, ":termId", "11111111-1111-4111-8111-111111111111")
			path += "?academic_session_id=11111111-1111-4111-8111-111111111111&term_id=11111111-1111-4111-8111-111111111111"
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
