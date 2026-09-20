package service

import (
 "errors"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/gorm"
)

var ErrStudentPortalForbidden = errors.New("student portal access forbidden")
var ErrStudentPortalNotFound = errors.New("student portal record not found")

type StudentPortalService struct{ db *gorm.DB }
func NewStudentPortalService(db *gorm.DB)*StudentPortalService{return &StudentPortalService{db:db}}
type StudentPortalProfile struct{ID uuid.UUID `json:"id"`;UserID uuid.UUID `json:"user_id"`;Name string `json:"name"`;Email string `json:"email"`;AdmissionNumber string `json:"admission_number"`;Gender string `json:"gender"`;EnrollmentStatus string `json:"enrollment_status"`}
type StudentPortalEnrollment struct{ID uuid.UUID `json:"id"`;AcademicSessionID uuid.UUID `json:"academic_session_id"`;SessionName string `json:"session_name"`;ClassID uuid.UUID `json:"class_id"`;ClassName string `json:"class_name"`;SectionID uuid.UUID `json:"section_id"`;SectionName string `json:"section_name"`;Status string `json:"status"`}
func(s *StudentPortalService)student(uid uuid.UUID)(models.Student,error){var st models.Student;if e:=s.db.Where("user_id=?",uid).First(&st).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return st,ErrStudentPortalForbidden};return st,e};return st,nil}
func(s *StudentPortalService)Profile(uid uuid.UUID)(StudentPortalProfile,error){st,e:=s.student(uid);if e!=nil{return StudentPortalProfile{},e};var u models.User;if e=s.db.First(&u,"id=?",st.UserID).Error;e!=nil{return StudentPortalProfile{},e};return StudentPortalProfile{st.ID,st.UserID,u.Name,u.Email,st.AdmissionNumber,st.Gender,st.EnrollmentStatus},nil}
func(s *StudentPortalService)Enrollment(uid uuid.UUID)(StudentPortalEnrollment,error){st,e:=s.student(uid);if e!=nil{return StudentPortalEnrollment{},e};var en models.StudentEnrollment;if e=s.db.Preload("AcademicSession").Preload("SchoolClass").Preload("Section").Where("student_id=? AND status=?",st.ID,models.EnrollmentStatusActive).Order("created_at DESC").First(&en).Error;e!=nil{return StudentPortalEnrollment{},ErrStudentPortalNotFound};return StudentPortalEnrollment{en.ID,en.AcademicSessionID,en.AcademicSession.Name,en.ClassID,en.SchoolClass.Name,en.SectionID,en.Section.Name,en.Status},nil}
func(s *StudentPortalService)Terms(uid uuid.UUID)([]GuardianTerm,error){en,e:=s.Enrollment(uid);if e!=nil{return nil,e};var terms []models.Term;if e=s.db.Where("academic_session_id=?",en.AcademicSessionID).Order("start_date").Find(&terms).Error;e!=nil{return nil,e};out:=make([]GuardianTerm,0,len(terms));for _,t:=range terms{out=append(out,GuardianTerm{t.ID,t.Name,t.AcademicSessionID,t.StartDate.Format("2006-01-02"),t.EndDate.Format("2006-01-02")})};return out,nil}
func(s *StudentPortalService)Attendance(uid uuid.UUID)([]models.AttendanceRecord,error){st,e:=s.student(uid);if e!=nil{return nil,e};var en models.StudentEnrollment;if e=s.db.Where("student_id=? AND status=?",st.ID,models.EnrollmentStatusActive).Order("created_at DESC").First(&en).Error;e!=nil{return []models.AttendanceRecord{},nil};var out []models.AttendanceRecord;e=s.db.Where("enrollment_id=?",en.ID).Preload("Term").Order("date DESC").Find(&out).Error;return out,e}
func(s *StudentPortalService)Timetable(uid uuid.UUID)([]models.TimetableEntry,error){en,e:=s.Enrollment(uid);if e!=nil{return nil,e};var out []models.TimetableEntry;e=s.db.Where("academic_session_id=? AND class_id=? AND active=? AND (section_id IS NULL OR section_id=?)",en.AcademicSessionID,en.ClassID,true,en.SectionID).Order("day_of_week,start_time").Find(&out).Error;return out,e}
func(s *StudentPortalService)ReportCard(uid,termID uuid.UUID)(ReportCard,error){en,e:=s.Enrollment(uid);if e!=nil{return ReportCard{},e};var t models.Term;if e=s.db.Where("id=? AND academic_session_id=?",termID,en.AcademicSessionID).First(&t).Error;e!=nil{return ReportCard{},ErrStudentPortalNotFound};return NewAssessmentResultService(nil,s.db).ReportCard(en.ID,t.ID)}
func(s *StudentPortalService)Invoices(uid uuid.UUID)([]models.Invoice,error){en,e:=s.Enrollment(uid);if e!=nil{return nil,e};var out []models.Invoice;e=s.db.Where("student_enrollment_id=?",en.ID).Preload("Term").Order("created_at DESC").Find(&out).Error;return out,e}
func(s *StudentPortalService)Payments(uid uuid.UUID)([]models.Payment,error){en,e:=s.Enrollment(uid);if e!=nil{return nil,e};var ids []uuid.UUID;s.db.Model(&models.Invoice{}).Where("student_enrollment_id=?",en.ID).Pluck("id",&ids);if len(ids)==0{return []models.Payment{},nil};var out []models.Payment;e=s.db.Where("invoice_id IN ?",ids).Order("created_at DESC").Find(&out).Error;return out,e}
