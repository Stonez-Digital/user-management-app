package controller

import (
	"errors"
	"net/http"
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
	TeacherID uuid.UUID `json:"teacher_id" binding:"required"`
	SubjectID uuid.UUID `json:"subject_id" binding:"required"`
	AcademicSessionID uuid.UUID `json:"academic_session_id" binding:"required"`
	TermID uuid.UUID `json:"term_id" binding:"required"`
	ClassID uuid.UUID `json:"class_id" binding:"required"`
	SectionID *uuid.UUID `json:"section_id"`
	AllocationType string `json:"allocation_type"`
	Active *bool `json:"active"`
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
func (ctrl *TeacherAssignmentController) Create(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return};var r teacherAssignmentRequest;if e:=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};active:=true;if r.Active!=nil{active=*r.Active};allocationType:=r.AllocationType;if allocationType==""{allocationType=models.TeacherAllocationSubject};v,e:=ctrl.service.Create(schoolID,models.TeacherAssignment{TeacherID:r.TeacherID,SubjectID:r.SubjectID,AcademicSessionID:r.AcademicSessionID,TermID:r.TermID,ClassID:r.ClassID,SectionID:r.SectionID,AllocationType:allocationType,Active:active});if e!=nil{assignmentError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(ctrl.service.DB(),c,&actor,"teacher_assignment.create","teacher_assignment",&v.ID,nil);c.JSON(http.StatusCreated,v) }
func (ctrl *TeacherAssignmentController) Update(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return};id,e:=uuid.Parse(c.Param("id"));if e!=nil{httpx.Error(c,400,"invalid_assignment_id","invalid assignment id");return};var r teacherAssignmentRequest;if e=c.ShouldBindJSON(&r);e!=nil{httpx.Validation(c,httpx.ValidationErrors(e));return};v,e:=ctrl.service.Get(schoolID,id);if e!=nil{assignmentError(c,e);return};v.TeacherID,v.SubjectID,v.AcademicSessionID,v.TermID,v.ClassID,v.SectionID=r.TeacherID,r.SubjectID,r.AcademicSessionID,r.TermID,r.ClassID,r.SectionID;if r.AllocationType!=""{v.AllocationType=r.AllocationType};if r.Active!=nil{v.Active=*r.Active};if e=ctrl.service.Update(schoolID,v);e!=nil{assignmentError(c,e);return};actor,_:=uuid.Parse(c.GetString("user_id"));_=audit.Record(ctrl.service.DB(),c,&actor,"teacher_assignment.update","teacher_assignment",&v.ID,nil);c.JSON(http.StatusOK,v) }
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
