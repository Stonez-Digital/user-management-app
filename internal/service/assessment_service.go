package service
import("errors";"strings";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")
var(ErrAssessmentNotFound=errors.New("assessment not found");ErrAssessmentInvalid=errors.New("invalid assessment"))
type AssessmentService struct{repo repository.AssessmentRepository;db *gorm.DB}
func NewAssessmentService(r repository.AssessmentRepository,db *gorm.DB)*AssessmentService{return &AssessmentService{r,db}}
func(s *AssessmentService)DB()*gorm.DB{return s.db}
func(s *AssessmentService)validate(v models.Assessment)error{var a models.TeacherAssignment;if e:=s.db.First(&a,"id = ?",v.TeacherAssignmentID).Error;e!=nil{return ErrAssessmentInvalid};v.Title=strings.TrimSpace(v.Title);v.Type=strings.TrimSpace(v.Type);if v.Title==""||v.Type==""||v.MaxScore<=0||v.Weight<=0||v.Weight>100||v.AssessmentDate.IsZero(){return ErrAssessmentInvalid};return nil}
func(s *AssessmentService)Create(v models.Assessment)(models.Assessment,error){if e:=s.validate(v);e!=nil{return v,e};return s.repo.Create(v)}
func(s *AssessmentService)List()([]models.Assessment,error){return s.repo.List()}
func(s *AssessmentService)Get(id uuid.UUID)(models.Assessment,error){v,e:=s.repo.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrAssessmentNotFound};return v,e}
func(s *AssessmentService)Update(v models.Assessment)error{if _,e:=s.Get(v.ID);e!=nil{return e};if e:=s.validate(v);e!=nil{return e};return s.repo.Update(v)}
func(s *AssessmentService)Delete(id uuid.UUID)error{if _,e:=s.Get(id);e!=nil{return e};return s.repo.Delete(id)}