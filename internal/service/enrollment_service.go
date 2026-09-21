package service
import("errors";"strings";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")
var(ErrEnrollmentNotFound=errors.New("enrollment not found");ErrEnrollmentDuplicate=errors.New("student already enrolled in academic session");ErrEnrollmentStudentMissing=errors.New("student not found");ErrEnrollmentSessionMissing=errors.New("academic session not found");ErrEnrollmentClassMissing=errors.New("class not found");ErrEnrollmentSectionMissing=errors.New("section not found");ErrEnrollmentSectionMismatch=errors.New("section does not belong to class");ErrEnrollmentInvalidStatus=errors.New("invalid enrollment status");ErrEnrollmentSchoolMismatch=errors.New("related record belongs to another school");ErrEnrollmentInUse=errors.New("enrollment is already used by academic or financial records"))
type EnrollmentService struct{repo repository.EnrollmentRepository;db *gorm.DB}
func NewEnrollmentService(repo repository.EnrollmentRepository,db *gorm.DB)*EnrollmentService{return &EnrollmentService{repo,db}}
func(s *EnrollmentService)DB()*gorm.DB{return s.db}
func(s *EnrollmentService)Create(schoolID uuid.UUID,v models.StudentEnrollment)(models.StudentEnrollment,error){
 var student models.Student;if err:=s.db.Where("id = ? AND school_id = ?",v.StudentID,schoolID).First(&student).Error;errors.Is(err,gorm.ErrRecordNotFound){return v,ErrEnrollmentStudentMissing}else if err!=nil{return v,err}
 var session models.AcademicSession;if err:=s.db.Where("id = ? AND school_id = ?",v.AcademicSessionID,schoolID).First(&session).Error;errors.Is(err,gorm.ErrRecordNotFound){return v,ErrEnrollmentSessionMissing}else if err!=nil{return v,err}
 var class models.SchoolClass;if err:=s.db.Where("id = ? AND school_id = ?",v.ClassID,schoolID).First(&class).Error;errors.Is(err,gorm.ErrRecordNotFound){return v,ErrEnrollmentClassMissing}else if err!=nil{return v,err}
 var section models.Section;if err:=s.db.Where("id = ? AND school_id = ?",v.SectionID,schoolID).First(&section).Error;errors.Is(err,gorm.ErrRecordNotFound){return v,ErrEnrollmentSectionMissing}else if err!=nil{return v,err};if section.ClassID!=v.ClassID{return v,ErrEnrollmentSectionMismatch}
 v.Status=strings.ToLower(strings.TrimSpace(v.Status));if v.Status==""{v.Status=models.EnrollmentStatusActive};if v.Status!=models.EnrollmentStatusActive&&v.Status!=models.EnrollmentStatusCompleted&&v.Status!=models.EnrollmentStatusWithdrawn{return v,ErrEnrollmentInvalidStatus}
 var existing models.StudentEnrollment;err:=s.db.Where("school_id = ? AND student_id = ? AND academic_session_id = ?",schoolID,v.StudentID,v.AcademicSessionID).First(&existing).Error;if err==nil{return v,ErrEnrollmentDuplicate};if !errors.Is(err,gorm.ErrRecordNotFound){return v,err};return s.repo.Create(schoolID,v)
}
func(s *EnrollmentService)List(schoolID uuid.UUID)([]models.StudentEnrollment,error){return s.repo.List(schoolID)}
func(s *EnrollmentService)ListForTeacher(schoolID,teacherID uuid.UUID)([]models.StudentEnrollment,error){
 var items []models.StudentEnrollment
 err:=s.db.Where("student_enrollments.school_id = ? AND student_enrollments.status = ?",schoolID,models.EnrollmentStatusActive).
  Joins("JOIN teacher_assignments ta ON ta.school_id = student_enrollments.school_id AND ta.class_id = student_enrollments.class_id AND ta.academic_session_id = student_enrollments.academic_session_id AND ta.teacher_id = ? AND ta.active = ?",teacherID,true).
  Where("ta.section_id IS NULL OR ta.section_id = student_enrollments.section_id").
  Preload("Student").Preload("Student.User").Preload("Class").Preload("Section").Preload("AcademicSession").
  Find(&items).Error
 return items,err
}
func(s *EnrollmentService)Get(schoolID,id uuid.UUID)(models.StudentEnrollment,error){v,err:=s.repo.Get(schoolID,id);if errors.Is(err,gorm.ErrRecordNotFound){return v,ErrEnrollmentNotFound};return v,err}
func(s *EnrollmentService)Update(schoolID uuid.UUID,v models.StudentEnrollment)error{current,err:=s.Get(schoolID,v.ID);if err!=nil{return err};if v.Status!=models.EnrollmentStatusActive&&v.Status!=models.EnrollmentStatusCompleted&&v.Status!=models.EnrollmentStatusWithdrawn{return ErrEnrollmentInvalidStatus};if v.StudentID!=current.StudentID||v.AcademicSessionID!=current.AcademicSessionID||v.ClassID!=current.ClassID||v.SectionID!=current.SectionID{return ErrEnrollmentSchoolMismatch};return s.repo.Update(schoolID,v)}
func(s *EnrollmentService)Delete(schoolID,id uuid.UUID)error{
 if _,err:=s.Get(schoolID,id);err!=nil{return err}
 var attendanceCount, resultCount, invoiceCount int64
 if err:=s.db.Model(&models.AttendanceRecord{}).Where("school_id = ? AND enrollment_id = ?",schoolID,id).Count(&attendanceCount).Error;err!=nil{return err}
 if err:=s.db.Model(&models.AssessmentResult{}).Where("school_id = ? AND student_enrollment_id = ?",schoolID,id).Count(&resultCount).Error;err!=nil{return err}
 if err:=s.db.Model(&models.Invoice{}).Where("school_id = ? AND student_enrollment_id = ?",schoolID,id).Count(&invoiceCount).Error;err!=nil{return err}
 if attendanceCount>0||resultCount>0||invoiceCount>0{return ErrEnrollmentInUse}
 return s.repo.Delete(schoolID,id)
}
