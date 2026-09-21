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

type EnrollmentController struct{ service *service.EnrollmentService }
func NewEnrollmentController(s *service.EnrollmentService) *EnrollmentController { return &EnrollmentController{service: s} }

type enrollmentRequest struct {
	StudentID uuid.UUID `json:"student_id" binding:"required"`
	AcademicSessionID uuid.UUID `json:"academic_session_id" binding:"required"`
	ClassID uuid.UUID `json:"class_id" binding:"required"`
	SectionID uuid.UUID `json:"section_id" binding:"required"`
	Status string `json:"status" binding:"omitempty,oneof=active completed withdrawn"`
}

type updateEnrollmentRequest struct {
	Status string `json:"status" binding:"required,oneof=active completed withdrawn"`
}

func enrollmentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEnrollmentNotFound): httpx.Error(c, 404, "enrollment_not_found", "enrollment not found")
	case errors.Is(err, service.ErrEnrollmentDuplicate): httpx.Error(c, 409, "enrollment_duplicate", "student is already enrolled in this academic session")
	case errors.Is(err, service.ErrEnrollmentStudentMissing): httpx.Error(c, 404, "student_not_found", "student not found")
	case errors.Is(err, service.ErrEnrollmentSessionMissing): httpx.Error(c, 404, "academic_session_not_found", "academic session not found")
	case errors.Is(err, service.ErrEnrollmentClassMissing): httpx.Error(c, 404, "class_not_found", "class not found")
	case errors.Is(err, service.ErrEnrollmentSectionMissing): httpx.Error(c, 404, "section_not_found", "section not found")
	case errors.Is(err, service.ErrEnrollmentSectionMismatch): httpx.Error(c, 400, "section_class_mismatch", "section does not belong to selected class")
	case errors.Is(err, service.ErrEnrollmentInvalidStatus): httpx.Error(c, 400, "invalid_enrollment_status", "invalid enrollment status")
	case errors.Is(err, service.ErrEnrollmentInUse): httpx.Error(c, 409, "enrollment_in_use", "enrollment cannot be deleted because it is referenced by attendance, results or invoices")
	case errors.Is(err, service.ErrEnrollmentPromotionSource): httpx.Error(c, 400, "enrollment_not_eligible", "enrollment is not eligible for this placement workflow")
	default: httpx.Error(c, 500, "enrollment_operation_failed", "enrollment operation failed")
	}
}

func (ctrl *EnrollmentController) TeacherList(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return}
 teacherID,e:=uuid.Parse(c.GetString("user_id"));if e!=nil{httpx.Error(c,401,"invalid_user","invalid authenticated user");return}
 items,e:=ctrl.service.ListForTeacher(schoolID,teacherID);if e!=nil{enrollmentError(c,e);return};c.JSON(http.StatusOK,items)
}
func (ctrl *EnrollmentController) History(c *gin.Context) {
 schoolID,ok:=requireSchoolID(c);if !ok{return}
 studentID,err:=uuid.Parse(c.Param("studentId"));if err!=nil{httpx.Error(c,400,"invalid_student_id","invalid student id");return}
 items,err:=ctrl.service.History(schoolID,studentID);if err!=nil{enrollmentError(c,err);return}
 c.JSON(http.StatusOK,gin.H{"enrollments":items})
}
type enrollmentPlacementRequest struct {
	TargetSessionID uuid.UUID `json:"target_session_id" binding:"required"`
	TargetClassID uuid.UUID `json:"target_class_id" binding:"required"`
	TargetSectionID uuid.UUID `json:"target_section_id" binding:"required"`
	Operation string `json:"operation" binding:"required,oneof=promote reenroll"`
}

func (ctrl *EnrollmentController) Place(c *gin.Context) {
	schoolID, ok := requireSchoolID(c); if !ok { return }
	id, err := uuid.Parse(c.Param("id")); if err != nil { httpx.Error(c,400,"invalid_enrollment_id","invalid enrollment id"); return }
	var req enrollmentPlacementRequest
	if err := c.ShouldBindJSON(&req); err != nil { httpx.Validation(c,httpx.ValidationErrors(err)); return }
	item, err := ctrl.service.Place(schoolID,id,service.EnrollmentPlacementRequest{TargetSessionID:req.TargetSessionID,TargetClassID:req.TargetClassID,TargetSectionID:req.TargetSectionID,Operation:req.Operation})
	if err != nil { enrollmentError(c,err); return }
	actor,_:=uuid.Parse(c.GetString("user_id")); _=audit.Record(ctrl.service.DB(),c,&actor,"enrollment.place","student_enrollment",&item.ID,nil)
	c.JSON(http.StatusCreated,item)
}
func (ctrl *EnrollmentController) List(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return}
	items, err := ctrl.service.List(schoolID)
	if err != nil { enrollmentError(c, err); return }
	c.JSON(http.StatusOK, items)
}
func (ctrl *EnrollmentController) Get(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, 400, "invalid_enrollment_id", "invalid enrollment id"); return }
	item, err := ctrl.service.Get(schoolID,id)
	if err != nil { enrollmentError(c, err); return }
	c.JSON(http.StatusOK, item)
}
func (ctrl *EnrollmentController) Create(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return}
	var req enrollmentRequest
	if err := c.ShouldBindJSON(&req); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }
	item, err := ctrl.service.Create(schoolID,models.StudentEnrollment{StudentID:req.StudentID, AcademicSessionID:req.AcademicSessionID, ClassID:req.ClassID, SectionID:req.SectionID, Status:req.Status})
	if err != nil { enrollmentError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actor, "enrollment.create", "student_enrollment", &item.ID, nil)
	c.JSON(http.StatusCreated, item)
}
func (ctrl *EnrollmentController) Update(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, 400, "invalid_enrollment_id", "invalid enrollment id"); return }
	var req updateEnrollmentRequest
	if err := c.ShouldBindJSON(&req); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }
	item, err := ctrl.service.Get(schoolID,id)
	if err != nil { enrollmentError(c, err); return }
	item.Status = req.Status
	if err := ctrl.service.Update(schoolID,item); err != nil { enrollmentError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actor, "enrollment.update", "student_enrollment", &item.ID, nil)
	c.JSON(http.StatusOK, item)
}
func (ctrl *EnrollmentController) Delete(c *gin.Context) { schoolID,ok:=requireSchoolID(c);if !ok{return}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, 400, "invalid_enrollment_id", "invalid enrollment id"); return }
	if err := ctrl.service.Delete(schoolID,id); err != nil { enrollmentError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actor, "enrollment.delete", "student_enrollment", &id, nil)
	c.JSON(http.StatusOK, gin.H{"message":"enrollment deleted"})
}
