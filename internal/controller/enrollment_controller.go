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
	default: httpx.Error(c, 500, "enrollment_operation_failed", "enrollment operation failed")
	}
}

func (ctrl *EnrollmentController) List(c *gin.Context) {
	items, err := ctrl.service.List()
	if err != nil { enrollmentError(c, err); return }
	c.JSON(http.StatusOK, items)
}
func (ctrl *EnrollmentController) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, 400, "invalid_enrollment_id", "invalid enrollment id"); return }
	item, err := ctrl.service.Get(id)
	if err != nil { enrollmentError(c, err); return }
	c.JSON(http.StatusOK, item)
}
func (ctrl *EnrollmentController) Create(c *gin.Context) {
	var req enrollmentRequest
	if err := c.ShouldBindJSON(&req); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }
	item, err := ctrl.service.Create(models.StudentEnrollment{StudentID:req.StudentID, AcademicSessionID:req.AcademicSessionID, ClassID:req.ClassID, SectionID:req.SectionID, Status:req.Status})
	if err != nil { enrollmentError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actor, "enrollment.create", "student_enrollment", &item.ID, nil)
	c.JSON(http.StatusCreated, item)
}
func (ctrl *EnrollmentController) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, 400, "invalid_enrollment_id", "invalid enrollment id"); return }
	var req enrollmentRequest
	if err := c.ShouldBindJSON(&req); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }
	item, err := ctrl.service.Get(id)
	if err != nil { enrollmentError(c, err); return }
	item.StudentID, item.AcademicSessionID, item.ClassID, item.SectionID = req.StudentID, req.AcademicSessionID, req.ClassID, req.SectionID
	if req.Status != "" { item.Status = req.Status }
	if err := ctrl.service.Update(item); err != nil { enrollmentError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actor, "enrollment.update", "student_enrollment", &item.ID, nil)
	c.JSON(http.StatusOK, item)
}
func (ctrl *EnrollmentController) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, 400, "invalid_enrollment_id", "invalid enrollment id"); return }
	if err := ctrl.service.Delete(id); err != nil { enrollmentError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(ctrl.service.DB(), c, &actor, "enrollment.delete", "student_enrollment", &id, nil)
	c.JSON(http.StatusOK, gin.H{"message":"enrollment deleted"})
}
