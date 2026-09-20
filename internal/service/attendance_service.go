package service
import("errors";"strings";"time";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")
var(ErrAttendanceNotFound=errors.New("attendance record not found");ErrAttendanceEnrollmentMissing=errors.New("enrollment not found");ErrAttendanceTermMissing=errors.New("term not found");ErrAttendanceTermMismatch=errors.New("term does not belong to enrollment academic session");ErrAttendanceDuplicate=errors.New("attendance already recorded for enrollment on date");ErrAttendanceInvalidStatus=errors.New("invalid attendance status"))
type AttendanceService struct{repo repository.AttendanceRepository;db *gorm.DB}
func NewAttendanceService(r repository.AttendanceRepository,db *gorm.DB)*AttendanceService{return &AttendanceService{r,db}}
func(s *AttendanceService)DB()*gorm.DB{return s.db}
func validAttendanceStatus(v string)bool{switch v{case models.AttendancePresent,models.AttendanceAbsent,models.AttendanceLate,models.AttendanceExcused:return true};return false}
func(s *AttendanceService)validate(schoolID uuid.UUID,v models.AttendanceRecord)error{if v.Date.IsZero(){return errors.New("attendance date is required")};var en models.StudentEnrollment;if e:=s.db.Where("id = ? AND school_id = ?",v.EnrollmentID,schoolID).First(&en).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return ErrAttendanceEnrollmentMissing};return e};if en.Status!=models.EnrollmentStatusActive{return ErrAttendanceEnrollmentMissing};var term models.Term;if e:=s.db.Where("id = ? AND school_id = ?",v.TermID,schoolID).First(&term).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return ErrAttendanceTermMissing};return e};if term.AcademicSessionID!=en.AcademicSessionID{return ErrAttendanceTermMismatch};day:=time.Date(v.Date.Year(),v.Date.Month(),v.Date.Day(),0,0,0,0,time.UTC);start:=time.Date(term.StartDate.Year(),term.StartDate.Month(),term.StartDate.Day(),0,0,0,0,time.UTC);end:=time.Date(term.EndDate.Year(),term.EndDate.Month(),term.EndDate.Day(),0,0,0,0,time.UTC);if day.Before(start)||day.After(end){return ErrAttendanceTermMismatch};v.Status=strings.ToLower(strings.TrimSpace(v.Status));if !validAttendanceStatus(v.Status){return ErrAttendanceInvalidStatus};return nil}
func(s *AttendanceService)Create(schoolID uuid.UUID,v models.AttendanceRecord)(models.AttendanceRecord,error){if v.Date.IsZero(){return v,errors.New("attendance date is required")};v.Date=time.Date(v.Date.Year(),v.Date.Month(),v.Date.Day(),0,0,0,0,time.UTC);if e:=s.validate(schoolID,v);e!=nil{return v,e};var x models.AttendanceRecord;e:=s.db.Where("school_id = ? AND enrollment_id = ? AND date = ?",schoolID,v.EnrollmentID,v.Date).First(&x).Error;if e==nil{return v,ErrAttendanceDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};v.Status=strings.ToLower(strings.TrimSpace(v.Status));v.SchoolID=schoolID;return s.repo.Create(schoolID,v)}
func(s *AttendanceService) ListForTeacher(schoolID, teacherID uuid.UUID) ([]models.AttendanceRecord,error) {
 var rows []models.AttendanceRecord
 err:=s.db.Where("attendance_records.school_id = ? AND teacher_assignments.teacher_id = ?",schoolID,teacherID).
  Joins("JOIN student_enrollments ON student_enrollments.id = attendance_records.enrollment_id").
  Joins("JOIN teacher_assignments ON teacher_assignments.school_id = attendance_records.school_id AND teacher_assignments.class_id = student_enrollments.class_id AND teacher_assignments.academic_session_id = student_enrollments.academic_session_id").
  Preload("Enrollment").Preload("Term").Find(&rows).Error
 return rows,err
}
func(s *AttendanceService) TeacherCreate(schoolID,teacherID uuid.UUID,v models.AttendanceRecord)(models.AttendanceRecord,error){
 var en models.StudentEnrollment
 if e:=s.db.Where("id = ? AND school_id = ?",v.EnrollmentID,schoolID).First(&en).Error;e!=nil{return v,ErrAttendanceEnrollmentMissing}
 var a models.TeacherAssignment
 q:=s.db.Where("school_id = ? AND teacher_id = ? AND active = ? AND class_id = ? AND academic_session_id = ? AND term_id = ?",schoolID,teacherID,true,en.ClassID,en.AcademicSessionID,v.TermID)
 if en.SectionID != uuid.Nil { q=q.Where("section_id = ? OR section_id = ?",en.SectionID,uuid.Nil) }
 if e:=q.First(&a).Error;e!=nil{return v,ErrAttendanceEnrollmentMissing}
 return s.Create(schoolID,v)
}
func(s *AttendanceService)Get(schoolID,id uuid.UUID)(models.AttendanceRecord,error){v,e:=s.repo.Get(schoolID,id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrAttendanceNotFound};return v,e}
func(s *AttendanceService)Update(schoolID uuid.UUID,v models.AttendanceRecord)error{cur,e:=s.Get(schoolID,v.ID);if e!=nil{return e};if e=s.validate(schoolID,v);e!=nil{return e};v.Date=time.Date(v.Date.Year(),v.Date.Month(),v.Date.Day(),0,0,0,0,time.UTC);if cur.EnrollmentID!=v.EnrollmentID||!cur.Date.Equal(v.Date){var x models.AttendanceRecord;if e=s.db.Where("school_id = ? AND enrollment_id = ? AND date = ? AND id <> ?",schoolID,v.EnrollmentID,v.Date,v.ID).First(&x).Error;e==nil{return ErrAttendanceDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return e}};v.Status=strings.ToLower(strings.TrimSpace(v.Status));v.SchoolID=schoolID;return s.repo.Update(schoolID,v)}
func(s *AttendanceService)Delete(schoolID,id uuid.UUID)error{if _,e:=s.Get(schoolID,id);e!=nil{return e};return s.repo.Delete(schoolID,id)}
