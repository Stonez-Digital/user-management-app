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

type AttendanceController struct{ service *service.AttendanceService }

func NewAttendanceController(s *service.AttendanceService) *AttendanceController {
	return &AttendanceController{service: s}
}

func (ctrl *AttendanceController) requestService(c *gin.Context) (*service.AttendanceService, bool) {
	tx, ok := middleware.TenantDBFromContext(c)
	if !ok {
		httpx.Error(c, http.StatusInternalServerError, "tenant_transaction_required", "tenant transaction is required for this operation")
		return nil, false
	}
	return ctrl.service.WithDB(tx), true
}

func parseAttendanceDate(v string) (time.Time, error) { return time.Parse("2006-01-02", v) }

type attendanceRequest struct {
	EnrollmentID uuid.UUID `json:"enrollment_id"`
	TermID       uuid.UUID `json:"term_id"`
	Date         string    `json:"date"`
	Status       string    `json:"status"`
	Note         string    `json:"note"`
}

func attendanceError(c *gin.Context, e error) {
	switch {
	case errors.Is(e, service.ErrAttendanceNotFound):
		httpx.Error(c, http.StatusNotFound, "attendance_not_found", "attendance record not found")
	case errors.Is(e, service.ErrAttendanceDuplicate):
		httpx.Error(c, http.StatusConflict, "attendance_duplicate", "attendance already recorded for this enrollment on this date")
	case errors.Is(e, service.ErrAttendanceEnrollmentMissing):
		httpx.Error(c, http.StatusNotFound, "enrollment_not_found", "active enrollment not found")
	case errors.Is(e, service.ErrAttendanceTermMissing):
		httpx.Error(c, http.StatusNotFound, "term_not_found", "term not found")
	case errors.Is(e, service.ErrAttendanceTermMismatch):
		httpx.Error(c, http.StatusBadRequest, "term_session_mismatch", "term does not belong to enrollment academic session")
	case errors.Is(e, service.ErrAttendanceInvalidStatus):
		httpx.Error(c, http.StatusBadRequest, "invalid_attendance_status", "invalid attendance status")
	default:
		httpx.Error(c, http.StatusInternalServerError, "attendance_operation_failed", "attendance operation failed")
	}
}

func (ctrl *AttendanceController) List(c *gin.Context) {
	svc, ok := ctrl.requestService(c); if !ok { return }
	schoolID, ok := requireSchoolID(c); if !ok { return }
	sessionID, termID, ok := parseAcademicQuery(c); if !ok { return }
	v, err := svc.List(schoolID, sessionID, termID)
	if err != nil { attendanceError(c, err); return }
	c.JSON(http.StatusOK, v)
}

func (ctrl *AttendanceController) Get(c *gin.Context) {
	svc, ok := ctrl.requestService(c); if !ok { return }
	schoolID, ok := requireSchoolID(c); if !ok { return }
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_attendance_id", "invalid attendance id"); return }
	v, err := svc.Get(schoolID, id)
	if err != nil { attendanceError(c, err); return }
	c.JSON(http.StatusOK, v)
}

func (ctrl *AttendanceController) Create(c *gin.Context) {
	svc, ok := ctrl.requestService(c); if !ok { return }
	var req attendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.EnrollmentID == uuid.Nil || req.TermID == uuid.Nil || req.Date == "" || req.Status == "" {
		httpx.Error(c, http.StatusBadRequest, "invalid_attendance_request", "enrollment, term, date and status are required"); return
	}
	date, err := parseAttendanceDate(req.Date)
	if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_attendance_date", "date must use YYYY-MM-DD format"); return }
	schoolID, ok := requireSchoolID(c); if !ok { return }
	v, err := svc.Create(schoolID, models.AttendanceRecord{EnrollmentID:req.EnrollmentID, TermID:req.TermID, Date:date, Status:req.Status, Note:req.Note})
	if err != nil { attendanceError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(svc.DB(), c, &actor, "attendance.create", "attendance_record", &v.ID, nil)
	c.JSON(http.StatusCreated, v)
}

func (ctrl *AttendanceController) Update(c *gin.Context) {
	svc, ok := ctrl.requestService(c); if !ok { return }
	schoolID, ok := requireSchoolID(c); if !ok { return }
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_attendance_id", "invalid attendance id"); return }
	var req attendanceRequest
	if err = c.ShouldBindJSON(&req); err != nil || req.EnrollmentID == uuid.Nil || req.TermID == uuid.Nil || req.Date == "" || req.Status == "" {
		httpx.Error(c, http.StatusBadRequest, "invalid_attendance_request", "enrollment, term, date and status are required"); return
	}
	date, err := parseAttendanceDate(req.Date)
	if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_attendance_date", "date must use YYYY-MM-DD format"); return }
	v, err := svc.Get(schoolID, id)
	if err != nil { attendanceError(c, err); return }
	v.EnrollmentID, v.TermID, v.Date, v.Status, v.Note = req.EnrollmentID, req.TermID, date, req.Status, req.Note
	if err = svc.Update(schoolID, v); err != nil { attendanceError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(svc.DB(), c, &actor, "attendance.update", "attendance_record", &v.ID, nil)
	c.JSON(http.StatusOK, v)
}

func (ctrl *AttendanceController) Delete(c *gin.Context) {
	svc, ok := ctrl.requestService(c); if !ok { return }
	schoolID, ok := requireSchoolID(c); if !ok { return }
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_attendance_id", "invalid attendance id"); return }
	if err = svc.Delete(schoolID, id); err != nil { attendanceError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(svc.DB(), c, &actor, "attendance.delete", "attendance_record", &id, nil)
	c.JSON(http.StatusOK, gin.H{"message":"attendance record deleted"})
}

func (ctrl *AttendanceController) TeacherList(c *gin.Context) {
	svc, ok := ctrl.requestService(c); if !ok { return }
	schoolID, ok := requireSchoolID(c); if !ok { return }
	teacherID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil { httpx.Error(c, http.StatusUnauthorized, "invalid_user", "invalid authenticated user"); return }
	sessionID, termID, ok := parseAcademicQuery(c); if !ok { return }
	v, err := svc.ListForTeacher(schoolID, teacherID, sessionID, termID)
	if err != nil { attendanceError(c, err); return }
	c.JSON(http.StatusOK, v)
}

func (ctrl *AttendanceController) TeacherCreate(c *gin.Context) {
	svc, ok := ctrl.requestService(c); if !ok { return }
	var req attendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.EnrollmentID == uuid.Nil || req.TermID == uuid.Nil || req.Date == "" || req.Status == "" {
		httpx.Error(c, http.StatusBadRequest, "invalid_attendance_request", "enrollment, term, date and status are required"); return
	}
	date, err := parseAttendanceDate(req.Date)
	if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_attendance_date", "date must use YYYY-MM-DD format"); return }
	schoolID, ok := requireSchoolID(c); if !ok { return }
	teacherID, err := uuid.Parse(c.GetString("user_id"))
	if err != nil { httpx.Error(c, http.StatusUnauthorized, "invalid_user", "invalid authenticated user"); return }
	v, err := svc.TeacherCreate(schoolID, teacherID, models.AttendanceRecord{EnrollmentID:req.EnrollmentID, TermID:req.TermID, Date:date, Status:req.Status, Note:req.Note})
	if err != nil { attendanceError(c, err); return }
	actor, _ := uuid.Parse(c.GetString("user_id"))
	_ = audit.Record(svc.DB(), c, &actor, "attendance.create", "attendance_record", &v.ID, nil)
	c.JSON(http.StatusCreated, v)
}
