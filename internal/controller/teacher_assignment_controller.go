package controller

import (
	"errors"
	"net/http"
	"strings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/audit"
	"github.com/onoja217/users-management-app/internal/httpx"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
)

type TeacherAssignmentController struct{ service *service.TeacherAssignmentService }
func NewTeacherAssignmentController(s *service.TeacherAssignmentService) *TeacherAssignmentController { return &TeacherAssignmentController{s} }

type teacherAssignmentRequest struct {
	TeacherID string `json:"teacher_id"`
	SubjectID string `json:"subject_id"`
	AcademicSessionID string `json:"academic_session_id"`
	TermID string `json:"term_id"`
	ClassID string `json:"class_id"`
	SectionID *string `json:"section_id"`
	AllocationType string `json:"allocation_type"`
	Active *bool `json:"active"`
}

func (r teacherAssignmentRequest) toModel() (models.TeacherAssignment, []httpx.FieldError) {
	values := []struct { name, value string }{
		{"teacher_id", r.TeacherID}, {"subject_id", r.SubjectID},
		{"academic_session_id", r.AcademicSessionID}, {"term_id", r.TermID},
		{"class_id", r.ClassID},
	}
	ids := make([]uuid.UUID, len(values))
	var problems []httpx.FieldError
	for i, item := range values {
		id, err := uuid.Parse(item.value)
		if err != nil {
			rule := "invalid_uuid"
			if strings.TrimSpace(item.value) == "" { rule = "required" }
			problems = append(problems, httpx.FieldError{Field:item.name, Rule:rule})
			continue
		}
		ids[i] = id
	}
	var sectionID *uuid.UUID
	if r.SectionID != nil && strings.TrimSpace(*r.SectionID) != "" {
		id, err := uuid.Parse(*r.SectionID)
		if err != nil { problems = append(problems, httpx.FieldError{Field:"section_id", Rule:"invalid_uuid"}) } else { sectionID = &id }
	}
	if len(problems) > 0 { return models.TeacherAssignment{}, problems }
	allocationType := r.AllocationType
	if allocationType == "" { allocationType = models.TeacherAllocationSubject }
	active := true
	if r.Active != nil { active = *r.Active }
	return models.TeacherAssignment{
		TeacherID:ids[0], SubjectID:ids[1], AcademicSessionID:ids[2], TermID:ids[3],
		ClassID:ids[4], SectionID:sectionID, AllocationType:allocationType, Active:active,
	}, nil
}

func assignmentError(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrAssignmentNotFound): httpx.Error(c,404,"assignment_not_found","teacher assignment not found")
	case errors.Is(e, service.ErrAssignmentDuplicate): httpx.Error(c,409,"assignment_duplicate","teacher assignment already exists")
	case errors.Is(e, service.ErrAssignmentInvalid): httpx.Error(c,400,"invalid_assignment","teacher, subject, session, term, class or section is invalid")
	case errors.Is(e, service.ErrAssignmentInUse): httpx.Error(c,409,"assignment_in_use","teacher assignment cannot be deleted because it is referenced by assessments or timetable records")
	case errors.Is(e, service.ErrAssignmentConflict): httpx.Error(c,409,"assignment_conflict","a class/form teacher is already allocated for this class or section and term")
	default: httpx.Error(c,500,"assignment_operation_failed","teacher assignment operation failed")
	}
}
func (ctrl *TeacherAssignmentController) TeacherList(c *gin.Context) {
    schoolID,ok:=requireSchoolID(c);if !ok{return}
    teacherID,e:=uuid.Parse(c.GetString("user_id"));if e!=nil{httpx.Error(c,401,"invalid_user","invalid authenticated user");return}
    sessionID,termID,ok:=parseAcademicQuery(c);if !ok{return};v,e:=ctrl.service.ListForTeacher(schoolID,teacherID,sessionID,termID);if e!=nil{assignmentError(c,e);return}
    c.JSON(http.StatusOK,v)
}
func (ctrl *TeacherAssignmentController) List(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return};v,e:=ctrl.service.List(schoolID);if e!=nil{assignmentError(c,e);return};c.JSON(http.StatusOK,v) }
func (ctrl *TeacherAssignmentController) Get(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_assignment_id","invalid assignment id");return};v,e:=ctrl.service.Get(schoolID,id);if e!=nil{assignmentError(c,e);return};c.JSON(http.StatusOK,v) }
func (ctrl *TeacherAssignmentController) Create(c *gin.Context) {
	schoolID, ok := requireSchoolID(c); if !ok { return }
	var r teacherAssignmentRequest
	if err := c.ShouldBindJSON(&r); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }
	assignment, problems := r.toModel()
	if len(problems) > 0 { httpx.Validation(c, problems); return }
	v, err := ctrl.service.Create(schoolID, assignment)
	if err != nil { assignmentError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actor, "teacher_assignment.create", "teacher_assignment", &v.ID, nil)
	c.JSON(http.StatusCreated, v)
}
func (ctrl *TeacherAssignmentController) Update(c *gin.Context) {
	schoolID, ok := requireSchoolID(c); if !ok { return }
	id, err := uuid.Parse(c.Param("id")); if err != nil { httpx.Error(c, 400, "invalid_assignment_id", "invalid assignment id"); return }
	var r teacherAssignmentRequest
	if err = c.ShouldBindJSON(&r); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }
	assignment, problems := r.toModel()
	if len(problems) > 0 { httpx.Validation(c, problems); return }
	v, err := ctrl.service.Get(schoolID, id); if err != nil { assignmentError(c, err); return }
	v.TeacherID, v.SubjectID, v.AcademicSessionID, v.TermID, v.ClassID, v.SectionID = assignment.TeacherID, assignment.SubjectID, assignment.AcademicSessionID, assignment.TermID, assignment.ClassID, assignment.SectionID
	if r.AllocationType != "" { v.AllocationType = r.AllocationType }
	if r.Active != nil { v.Active = *r.Active }
	if err = ctrl.service.Update(schoolID, v); err != nil { assignmentError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actor, "teacher_assignment.update", "teacher_assignment", &v.ID, nil)
	c.JSON(http.StatusOK, v)
}
func (ctrl *TeacherAssignmentController) Delete(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_assignment_id","invalid assignment id");return};if e=ctrl.service.Delete(schoolID,id);e!=nil{assignmentError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(ctrl.service.DB(),c,&actor,"teacher_assignment.delete","teacher_assignment",&id,nil);c.JSON(http.StatusOK,gin.H{"message":"teacher assignment deleted"}) }

func (ctrl *TeacherAssignmentController) Coverage(c *gin.Context) {
	schoolID,ok:=requireSchoolID(c);if !ok{return}
	var sessionID,termID uuid.UUID
	var err error
	if raw:=c.Query("academic_session_id");raw!="" { sessionID,err=uuid.Parse(raw);if err!=nil{httpx.Error(c,400,"invalid_session_id","invalid academic session id");return} }
	if raw:=c.Query("term_id");raw!="" { termID,err=uuid.Parse(raw);if err!=nil{httpx.Error(c,400,"invalid_term_id","invalid term id");return} }
	v,e:=ctrl.service.Coverage(schoolID,sessionID,termID);if e!=nil{assignmentError(c,e);return}
	c.JSON(http.StatusOK,gin.H{"assignments":v})
}
