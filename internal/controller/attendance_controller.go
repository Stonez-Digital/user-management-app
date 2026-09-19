package controller

import (
    "errors"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/audit"
    "github.com/onoja217/users-management-app/internal/httpx"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/service"
)

type AttendanceController struct{ service *service.AttendanceService }
func NewAttendanceController(s *service.AttendanceService) *AttendanceController { return &AttendanceController{service:s} }

func parseAttendanceDate(value string) (time.Time, error) {
    return time.Parse("2006-01-02", value)
}

type attendanceRequest struct {
    EnrollmentID uuid.UUID `json:"enrollment_id"`
    TermID uuid.UUID `json:"term_id"`
    Date string `json:"date"`
    Status string `json:"status"`
    Note string `json:"note"`
}

func attendanceError(c *gin.Context, err error) {
    switch {
    case errors.Is(err, service.ErrAttendanceNotFound): httpx.Error(c,404,"attendance_not_found","attendance record not found")
    case errors.Is(err, service.ErrAttendanceDuplicate): httpx.Error(c,409,"attendance_duplicate","attendance already recorded for this enrollment on this date")
    case errors.Is(err, service.ErrAttendanceEnrollmentMissing): httpx.Error(c,404,"enrollment_not_found","active enrollment not found")
    case errors.Is(err, service.ErrAttendanceTermMissing): httpx.Error(c,404,"term_not_found","term not found")
    case errors.Is(err, service.ErrAttendanceTermMismatch): httpx.Error(c,400,"term_session_mismatch","term does not belong to enrollment academic session")
    case errors.Is(err, service.ErrAttendanceInvalidStatus): httpx.Error(c,400,"invalid_attendance_status","invalid attendance status")
    default: httpx.Error(c,500,"attendance_operation_failed","attendance operation failed")
    }
}

func (ctrl *AttendanceController) List(c *gin.Context) {
    items,err:=ctrl.service.List()
    if err!=nil { attendanceError(c,err); return }
    c.JSON(http.StatusOK,items)
}
func (ctrl *AttendanceController) Get(c *gin.Context) {
    id,err:=uuid.Parse(c.Param("id"))
    if err!=nil { httpx.Error(c,400,"invalid_attendance_id","invalid attendance id"); return }
    item,err:=ctrl.service.Get(id)
    if err!=nil { attendanceError(c,err); return }
    c.JSON(http.StatusOK,item)
}
func (ctrl *AttendanceController) Create(c *gin.Context) {
    var req attendanceRequest
    if err:=c.ShouldBindJSON(&req); err!=nil || req.EnrollmentID==uuid.Nil || req.TermID==uuid.Nil || req.Date=="" || req.Status=="" {
        httpx.Error(c,400,"invalid_attendance_request","enrollment, term, date and status are required"); return
    }
    date,err:=parseAttendanceDate(req.Date)
    if err!=nil { httpx.Error(c,400,"invalid_attendance_date","date must use YYYY-MM-DD format"); return }
    item,err:=ctrl.service.Create(models.AttendanceRecord{EnrollmentID:req.EnrollmentID,TermID:req.TermID,Date:date,Status:req.Status,Note:req.Note})
    if err!=nil { attendanceError(c,err); return }
    actor,_:=uuid.Parse(c.GetString("user_id"))
    _=audit.Record(ctrl.service.DB(),c,&actor,"attendance.create","attendance_record",&item.ID,nil)
    c.JSON(http.StatusCreated,item)
}
func (ctrl *AttendanceController) Update(c *gin.Context) {
    id,err:=uuid.Parse(c.Param("id"))
    if err!=nil { httpx.Error(c,400,"invalid_attendance_id","invalid attendance id"); return }
    var req attendanceRequest
    if err:=c.ShouldBindJSON(&req); err!=nil || req.EnrollmentID==uuid.Nil || req.TermID==uuid.Nil || req.Date=="" || req.Status=="" {
        httpx.Error(c,400,"invalid_attendance_request","enrollment, term, date and status are required"); return
    }
    date,err:=parseAttendanceDate(req.Date)
    if err!=nil { httpx.Error(c,400,"invalid_attendance_date","date must use YYYY-MM-DD format"); return }
    item,err:=ctrl.service.Get(id)
    if err!=nil { attendanceError(c,err); return }
    item.EnrollmentID=req.EnrollmentID; item.TermID=req.TermID; item.Date=date; item.Status=req.Status; item.Note=req.Note
    if err:=ctrl.service.Update(item); err!=nil { attendanceError(c,err); return }
    actor,_:=uuid.Parse(c.GetString("user_id"))
    _=audit.Record(ctrl.service.DB(),c,&actor,"attendance.update","attendance_record",&item.ID,nil)
    c.JSON(http.StatusOK,item)
}
func (ctrl *AttendanceController) Delete(c *gin.Context) {
    id,err:=uuid.Parse(c.Param("id"))
    if err!=nil { httpx.Error(c,400,"invalid_attendance_id","invalid attendance id"); return }
    if err:=ctrl.service.Delete(id); err!=nil { attendanceError(c,err); return }
    actor,_:=uuid.Parse(c.GetString("user_id"))
    _=audit.Record(ctrl.service.DB(),c,&actor,"attendance.delete","attendance_record",&id,nil)
    c.JSON(http.StatusOK,gin.H{"message":"attendance record deleted"})
}
