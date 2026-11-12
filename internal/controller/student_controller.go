package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/audit"
	"github.com/onoja217/users-management-app/internal/httpx"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
)

type StudentController struct {
	service *service.StudentService
}

func NewStudentController(s *service.StudentService) *StudentController {
	return &StudentController{service: s}
}

type CreateStudentRequest struct {
	UserID           uuid.UUID  `json:"user_id" binding:"required"`
	AdmissionNumber  string     `json:"admission_number" binding:"required,min=2,max=50"`
	DateOfBirth      *time.Time `json:"date_of_birth"`
	Gender           string     `json:"gender" binding:"omitempty,max=30"`
	GuardianName     string     `json:"guardian_name" binding:"omitempty,max=100"`
	GuardianPhone    string     `json:"guardian_phone" binding:"omitempty,max=30"`
	GuardianEmail    string     `json:"guardian_email" binding:"omitempty,email,max=255"`
	EnrollmentStatus string     `json:"enrollment_status" binding:"omitempty,oneof=active inactive graduated withdrawn"`
}

type UpdateStudentRequest struct {
	AdmissionNumber  string     `json:"admission_number" binding:"omitempty,min=2,max=50"`
	DateOfBirth      *time.Time `json:"date_of_birth"`
	Gender           string     `json:"gender" binding:"omitempty,max=30"`
	GuardianName     string     `json:"guardian_name" binding:"omitempty,max=100"`
	GuardianPhone    string     `json:"guardian_phone" binding:"omitempty,max=30"`
	GuardianEmail    string     `json:"guardian_email" binding:"omitempty,email,max=255"`
	EnrollmentStatus string     `json:"enrollment_status" binding:"omitempty,oneof=active inactive graduated withdrawn"`
}

// requestService fails closed unless the route explicitly installed the tenant
// transaction middleware. The fallback shared DB handle must never be used here.
func (ctrl *StudentController) requestService(c *gin.Context) (*service.StudentService, bool) {
	tx, ok := middleware.TenantDBFromContext(c)
	if !ok {
		httpx.Error(c, http.StatusInternalServerError, "tenant_transaction_required", "tenant transaction is required for this operation")
		return nil, false
	}
	return ctrl.service.WithDB(tx), true
}

func recordStudentAudit(c *gin.Context, svc *service.StudentService, action string, resourceID *uuid.UUID) bool {
	actorID, err := uuid.Parse(c.GetString(middleware.UserIDKey))
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, "invalid_user_context", "invalid user context")
		return false
	}
	if err := audit.Record(svc.DB(), c, &actorID, action, "student", resourceID, nil); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "student_audit_failed", "failed to record student activity")
		return false
	}
	return true
}

func (ctrl *StudentController) Create(c *gin.Context) {
	var req CreateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err))
		return
	}
	schoolID, ok := requireSchoolID(c)
	if !ok {
		return
	}
	svc, ok := ctrl.requestService(c)
	if !ok {
		return
	}
	student, err := svc.CreateStudent(schoolID, models.Student{
		UserID: req.UserID, AdmissionNumber: req.AdmissionNumber, DateOfBirth: req.DateOfBirth,
		Gender: req.Gender, GuardianName: req.GuardianName, GuardianPhone: req.GuardianPhone,
		GuardianEmail: req.GuardianEmail, EnrollmentStatus: req.EnrollmentStatus,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrStudentUserNotFound):
			httpx.Error(c, http.StatusNotFound, "student_user_not_found", "student user not found")
		case errors.Is(err, service.ErrStudentUserRole):
			httpx.Error(c, http.StatusBadRequest, "invalid_student_user", "user must have the student role")
		case errors.Is(err, service.ErrDuplicateAdmission), errors.Is(err, service.ErrDuplicateStudentUser):
			httpx.Error(c, http.StatusConflict, "student_already_exists", "student record already exists")
		default:
			httpx.Error(c, http.StatusInternalServerError, "student_create_failed", "failed to create student")
		}
		return
	}
	if !recordStudentAudit(c, svc, "student.create", &student.ID) {
		return
	}
	c.JSON(http.StatusCreated, student)
}

func (ctrl *StudentController) List(c *gin.Context) {
	schoolID, ok := requireSchoolID(c)
	if !ok {
		return
	}
	svc, ok := ctrl.requestService(c)
	if !ok {
		return
	}
	students, err := svc.GetStudents(schoolID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "student_list_failed", "failed to load students")
		return
	}
	c.JSON(http.StatusOK, students)
}

func (ctrl *StudentController) Get(c *gin.Context) {
	schoolID, ok := requireSchoolID(c)
	if !ok {
		return
	}
	svc, ok := ctrl.requestService(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_student_id", "invalid student id")
		return
	}
	student, err := svc.GetStudent(schoolID, id)
	if errors.Is(err, service.ErrStudentNotFound) {
		httpx.Error(c, http.StatusNotFound, "student_not_found", "student not found")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "student_get_failed", "failed to load student")
		return
	}
	c.JSON(http.StatusOK, student)
}

func (ctrl *StudentController) Update(c *gin.Context) {
	schoolID, ok := requireSchoolID(c)
	if !ok {
		return
	}
	svc, ok := ctrl.requestService(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_student_id", "invalid student id")
		return
	}
	var req UpdateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err))
		return
	}
	student, err := svc.GetStudent(schoolID, id)
	if errors.Is(err, service.ErrStudentNotFound) {
		httpx.Error(c, http.StatusNotFound, "student_not_found", "student not found")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "student_get_failed", "failed to load student")
		return
	}
	if req.AdmissionNumber != "" { student.AdmissionNumber = req.AdmissionNumber }
	if req.DateOfBirth != nil { student.DateOfBirth = req.DateOfBirth }
	if req.Gender != "" { student.Gender = req.Gender }
	if req.GuardianName != "" { student.GuardianName = req.GuardianName }
	if req.GuardianPhone != "" { student.GuardianPhone = req.GuardianPhone }
	if req.GuardianEmail != "" { student.GuardianEmail = req.GuardianEmail }
	if req.EnrollmentStatus != "" { student.EnrollmentStatus = req.EnrollmentStatus }
	if err := svc.UpdateStudent(schoolID, student); err != nil {
		switch {
		case errors.Is(err, service.ErrDuplicateAdmission):
			httpx.Error(c, http.StatusConflict, "duplicate_admission_number", "admission number already exists")
		case errors.Is(err, service.ErrStudentNotFound):
			httpx.Error(c, http.StatusNotFound, "student_not_found", "student not found")
		default:
			httpx.Error(c, http.StatusInternalServerError, "student_update_failed", "failed to update student")
		}
		return
	}
	if !recordStudentAudit(c, svc, "student.update", &student.ID) {
		return
	}
	c.JSON(http.StatusOK, student)
}

func (ctrl *StudentController) Delete(c *gin.Context) {
	schoolID, ok := requireSchoolID(c)
	if !ok {
		return
	}
	svc, ok := ctrl.requestService(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_student_id", "invalid student id")
		return
	}
	if err := svc.DeleteStudent(schoolID, id); err != nil {
		if errors.Is(err, service.ErrStudentNotFound) {
			httpx.Error(c, http.StatusNotFound, "student_not_found", "student not found")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, "student_delete_failed", "failed to delete student")
		return
	}
	if !recordStudentAudit(c, svc, "student.delete", &id) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "student deleted"})
}
