package service
import("errors";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")
var(ErrEnrollmentNotFound=errors.New("enrollment not found");ErrEnrollmentDuplicate=errors.New("student already enrolled in session");ErrEnrollmentInvalid=errors.New("invalid enrollment"))
type EnrollmentService struct{repo repository.EnrollmentRepository;db *gorm.DB}
func NewEnrollmentService(r repository.EnrollmentRepository,db *gorm.DB)*EnrollmentService{return &EnrollmentService{r,db}}
func(s *EnrollmentService)DB()*gorm.DB{return s.db}
func(s *EnrollmentService)validate(v models.StudentEnrollment)error{
 var st models.Student;if e:=s.db.First(&st,"id = ?",v.StudentID).Error;e!=nil{return ErrEnrollmentInvalid}
 var session models.AcademicSession;if e:=s.db.First(&session,"id = ?",v.AcademicSessionID).Error;e!=nil{return ErrEnrollmentInvalid}
 var cl models.SchoolClass;if e:=s.db.First(&cl,"id = ?",v.ClassID).Error;e!=nil{return ErrEnrollmentInvalid}
 var sec models.Section;if e:=s.db.First(&sec,"id = ? AND class_id = ?",v.SectionID,v.ClassID).Error;e!=nil{return ErrEnrollmentInvalid}
 if v.Status!=models.EnrollmentStatusActive&&v.Status!=models.EnrollmentStatusCompleted&&v.Status!=models.EnrollmentStatusWithdrawn{return ErrEnrollmentInvalid}
 return nil
}
func(s *EnrollmentService)Create(v models.StudentEnrollment)(models.StudentEnrollment,error){if e:=s.validate(v);e!=nil{return v,e};var x models.StudentEnrollment;if e:=s.db.Where("student_id=? AND academic_session_id=?",v.StudentID,v.AcademicSessionID).First(&x).Error;e==nil{return v,ErrEnrollmentDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};return s.repo.Create(v)}
func(s *EnrollmentService)List()([]models.StudentEnrollment,error){return s.repo.List()}
func(s *EnrollmentService)Get(id uuid.UUID)(models.StudentEnrollment,error){v,e:=s.repo.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrEnrollmentNotFound};return v,e}
func(s *EnrollmentService)Update(v models.StudentEnrollment)error{if _,e:=s.Get(v.ID);e!=nil{return e};if e:=s.validate(v);e!=nil{return e};return s.repo.Update(v)}
func(s *EnrollmentService)Delete(id uuid.UUID)error{if _,e:=s.Get(id);e!=nil{return e};return s.repo.Delete(id)}
