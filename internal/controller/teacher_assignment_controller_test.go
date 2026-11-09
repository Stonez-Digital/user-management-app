package controller

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func bindTeacherAssignmentRequest(body string) (teacherAssignmentRequest, error) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/teacher-assignments", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	var request teacherAssignmentRequest
	err := ctx.ShouldBindJSON(&request)
	return request, err
}

func validTeacherAssignmentBody() string {
	return `{"teacher_id":"` + uuid.NewString() + `","subject_id":"` + uuid.NewString() + `","academic_session_id":"` + uuid.NewString() + `","term_id":"` + uuid.NewString() + `","class_id":"` + uuid.NewString() + `","section_id":null,"allocation_type":"subject_teacher","active":true}`
}

func TestTeacherAssignmentRequestParsesSelectedUUIDs(t *testing.T) {
	request, err := bindTeacherAssignmentRequest(validTeacherAssignmentBody())
	if err != nil { t.Fatalf("valid JSON should bind: %v", err) }
	assignment, problems := request.toModel()
	if len(problems) != 0 { t.Fatalf("valid UUIDs should parse, got: %#v", problems) }
	if assignment.TeacherID == uuid.Nil || assignment.SubjectID == uuid.Nil || assignment.AcademicSessionID == uuid.Nil || assignment.TermID == uuid.Nil || assignment.ClassID == uuid.Nil {
		t.Fatal("all required assignment IDs should be populated")
	}
	if assignment.SectionID != nil { t.Fatal("null section_id should mean whole class") }
}

func TestTeacherAssignmentRequestReportsMissingRequiredIDs(t *testing.T) {
	request, err := bindTeacherAssignmentRequest(`{"subject_id":"` + uuid.NewString() + `","academic_session_id":"` + uuid.NewString() + `","term_id":"` + uuid.NewString() + `","class_id":"` + uuid.NewString() + `"}`)
	if err != nil { t.Fatalf("valid JSON should bind: %v", err) }
	_, problems := request.toModel()
	if len(problems) != 1 || problems[0].Field != "teacher_id" || problems[0].Rule != "required" {
		t.Fatalf("expected a teacher_id required error, got %#v", problems)
	}
}

func TestTeacherAssignmentRequestReportsMalformedUUIDByField(t *testing.T) {
	request, err := bindTeacherAssignmentRequest(strings.Replace(validTeacherAssignmentBody(), jsonUUIDPlaceholder(), "not-a-uuid", 1))
	if err != nil { t.Fatalf("valid JSON should bind: %v", err) }
	_, problems := request.toModel()
	if len(problems) != 1 || problems[0].Field != "teacher_id" || problems[0].Rule != "invalid_uuid" {
		t.Fatalf("expected a teacher_id invalid_uuid error, got %#v", problems)
	}
}

func jsonUUIDPlaceholder() string {
	// The first UUID in the fixture is the teacher ID.
	start := strings.Index(validTeacherAssignmentBody(), `"teacher_id":"`) + len(`"teacher_id":"`)
	return validTeacherAssignmentBody()[start : start+36]
}
