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

type StudentController struct { service *service.StudentService }
func NewStudentController(s *service.StudentService) *StudentController { return &StudentController{service: s} }

type CreateStudentRequest struct {
    UserID uuid.UUID `json:"user_id" binding:"required"`
    AdmissionNumber string `json:"admission_number" binding:"required,min=2,max=50"`
    DateOfBirth *time.Time `json:"date_of_birth"`
    Gender string `json:"gender" binding:"omitempty,max=30"`
    GuardianName string `json:"guardian_name" binding:"omitempty,max=100"`
    GuardianPhone string `json:"guardian_phone" binding:"omitempty,max=30"`
    GuardianEmail string `json:"guardian_email" binding:"omitempty,email,max=255"`
    EnrollmentStatus string `json:"enrollment_status" binding:"omitempty,oneof=active inactive graduated withdrawn"`
}
type UpdateStudentRequest struct {
    AdmissionNumber string `json:"admission_number" binding:"omitempty,min=2,max=50"`
    DateOfBirth *time.Time `json:"date_of_birth"`
    Gender string `json:"gender" binding:"omitempty,max=30"`
    GuardianName string `json:"guardian_name" binding:"omitempty,max=100"`
    GuardianPhone string `json:"guardian_phone" binding:"omitempty,max=30"`
    GuardianEmail string `json:"guardian_email" binding:"omitempty,email,max=255"`
    EnrollmentStatus string `json:"enrollment_status" binding:"omitempty,oneof=active inactive graduated withdrawn"`
}

func (ctrl *StudentController) Create(c *gin.Context) {
    var req CreateStudentRequest; if err := c.ShouldBindJSON(&req); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }
    schoolID, ok := requireSchoolID(c); if !ok { return }; student, err := ctrl.service.CreateStudent(schoolID, models.Student{UserID:req.UserID, AdmissionNumber:req.AdmissionNumber, DateOfBirth:req.DateOfBirth, Gender:req.Gender, GuardianName:req.GuardianName, GuardianPhone:req.GuardianPhone, GuardianEmail:req.GuardianEmail, EnrollmentStatus:req.EnrollmentStatus})
    if err != nil {
        switch {
        case errors.Is(err, service.ErrStudentUserNotFound): httpx.Error(c,http.StatusNotFound,"student_user_not_found","student user not found")
        case errors.Is(err, service.ErrStudentUserRole): httpx.Error(c,http.StatusBadRequest,"invalid_student_user","user must have the student role")
        case errors.Is(err, service.ErrDuplicateAdmission), errors.Is(err, service.ErrDuplicateStudentUser): httpx.Error(c,http.StatusConflict,"student_already_exists","student record already exists")
        default: httpx.Error(c,http.StatusInternalServerError,"student_create_failed","failed to create student") }; return
    }
    actorID, _ := uuid.Parse(c.GetString("user_id")); _ = audit.Record(ctrl.service.DB(),c,&actorID,"student.create","student",&student.ID,nil); c.JSON(http.StatusCreated,student)
}
func (ctrl *StudentController) List(c *gin.Context) { students,err:=ctrl.service.GetStudents(schoolID); if err!=nil { httpx.Error(c,500,"student_list_failed","failed to load students"); return }; c.JSON(http.StatusOK,students) }
func (ctrl *StudentController) Get(c *gin.Context) {
    id,err:=uuid.Parse(c.Param("id")); if err!=nil { httpx.Error(c,400,"invalid_student_id","invalid student id"); return }
    student,err:=ctrl.service.GetStudent(schoolID, id); if errors.Is(err,service.ErrStudentNotFound) { httpx.Error(c,404,"student_not_found","student not found"); return }; if err!=nil { httpx.Error(c,500,"student_get_failed","failed to load student"); return }; c.JSON(http.StatusOK,student)
}
func (ctrl *StudentController) Update(c *gin.Context) {
    id,err:=uuid.Parse(c.Param("id")); if err!=nil { httpx.Error(c,400,"invalid_student_id","invalid student id"); return }
    var req UpdateStudentRequest; if err:=c.ShouldBindJSON(&req); err!=nil { httpx.Validation(c,httpx.ValidationErrors(err)); return }
    student,err:=ctrl.service.GetStudent(schoolID, id); if errors.Is(err,service.ErrStudentNotFound) { httpx.Error(c,404,"student_not_found","student not found"); return }; if err!=nil { httpx.Error(c,500,"student_get_failed","failed to load student"); return }
    if req.AdmissionNumber!="" { student.AdmissionNumber=req.AdmissionNumber }; if req.DateOfBirth!=nil { student.DateOfBirth=req.DateOfBirth }; if req.Gender!="" { student.Gender=req.Gender }; if req.GuardianName!="" { student.GuardianName=req.GuardianName }; if req.GuardianPhone!="" { student.GuardianPhone=req.GuardianPhone }; if req.GuardianEmail!="" { student.GuardianEmail=req.GuardianEmail }; if req.EnrollmentStatus!="" { student.EnrollmentStatus=req.EnrollmentStatus }
    if err:=ctrl.service.UpdateStudent(schoolID, student); err!=nil { if errors.Is(err,service.ErrDuplicateAdmission) { httpx.Error(c,409,"duplicate_admission_number","admission number already exists"); return }; if errors.Is(err,service.ErrStudentNotFound) { httpx.Error(c,404,"student_not_found","student not found"); return }; httpx.Error(c,500,"student_update_failed","failed to update student"); return }
    actorID,_:=uuid.Parse(c.GetString("user_id")); _=audit.Record(ctrl.service.DB(),c,&actorID,"student.update","student",&student.ID,nil); c.JSON(http.StatusOK,student)
}
func (ctrl *StudentController) Delete(c *gin.Context) {
    id,err:=uuid.Parse(c.Param("id")); if err!=nil { httpx.Error(c,400,"invalid_student_id","invalid student id"); return }
    if err:=ctrl.service.DeleteStudent(schoolID, id); err!=nil { if errors.Is(err,service.ErrStudentNotFound) { httpx.Error(c,404,"student_not_found","student not found"); return }; httpx.Error(c,500,"student_delete_failed","failed to delete student"); return }
    actorID,_:=uuid.Parse(c.GetString("user_id")); _=audit.Record(ctrl.service.DB(),c,&actorID,"student.delete","student",&id,nil); c.JSON(http.StatusOK,gin.H{"message":"student deleted"})
}