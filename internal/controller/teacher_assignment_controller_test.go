package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func bindTeacherAssignmentRequest(body string) error {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/teacher-assignments", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	var request teacherAssignmentRequest
	return ctx.ShouldBindJSON(&request)
}

func TestTeacherAssignmentRequestBindsSelectedUUIDs(t *testing.T) {
	body := `{"teacher_id":"` + uuid.NewString() + `","subject_id":"` + uuid.NewString() + `","academic_session_id":"` + uuid.NewString() + `","term_id":"` + uuid.NewString() + `","class_id":"` + uuid.NewString() + `","section_id":null,"allocation_type":"subject_teacher","active":true}`
	if err := bindTeacherAssignmentRequest(body); err != nil {
		t.Fatalf("selected assignment IDs should bind: %v", err)
	}
}

func TestTeacherAssignmentRequestRejectsMissingRequiredID(t *testing.T) {
	body := `{"subject_id":"` + uuid.NewString() + `","academic_session_id":"` + uuid.NewString() + `","term_id":"` + uuid.NewString() + `","class_id":"` + uuid.NewString() + `","section_id":null,"allocation_type":"subject_teacher","active":true}`
	if err := bindTeacherAssignmentRequest(body); err == nil {
		t.Fatal("expected missing teacher_id to fail request validation")
	}
}
