package service
import("errors";"strings";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")
var(ErrSubjectNotFound=errors.New("subject not found");ErrSubjectDuplicate=errors.New("subject already exists");ErrSubjectInvalid=errors.New("subject code and name are required"))
type SubjectService struct{repo repository.SubjectRepository;db *gorm.DB}
func NewSubjectService(repo repository.SubjectRepository,db *gorm.DB)*SubjectService{return &SubjectService{repo,db}}
func(s *SubjectService)DB()*gorm.DB{return s.db}
func(s *SubjectService)norm(v models.Subject)(models.Subject,error){v.Code=strings.ToUpper(strings.TrimSpace(v.Code));v.Name=strings.TrimSpace(v.Name);v.Description=strings.TrimSpace(v.Description);if v.Code==""||v.Name==""{return v,ErrSubjectInvalid};return v,nil}
func(s *SubjectService)Create(schoolID uuid.UUID,v models.Subject)(models.Subject,error){var e error;if v,e=s.norm(v);e!=nil{return v,e};var x models.Subject;if e=s.db.Where("school_id = ? AND (code = ? OR name = ?)",schoolID,v.Code,v.Name).First(&x).Error;e==nil{return v,ErrSubjectDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};return s.repo.Create(schoolID,v)}
func(s *SubjectService)List(schoolID uuid.UUID)([]models.Subject,error){return s.repo.List(schoolID)}
func(s *SubjectService)Get(schoolID,id uuid.UUID)(models.Subject,error){v,e:=s.repo.Get(schoolID,id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrSubjectNotFound};return v,e}
func(s *SubjectService)Update(schoolID uuid.UUID,v models.Subject)error{if _,e:=s.Get(schoolID,v.ID);e!=nil{return e};var e error;if v,e=s.norm(v);e!=nil{return e};var x models.Subject;if e=s.db.Where("school_id = ? AND (code = ? OR name = ?) AND id <> ?",schoolID,v.Code,v.Name,v.ID).First(&x).Error;e==nil{return ErrSubjectDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return e};return s.repo.Update(schoolID,v)}
func(s *SubjectService)Delete(schoolID,id uuid.UUID)error{if _,e:=s.Get(schoolID,id);e!=nil{return e};return s.repo.Delete(schoolID,id)}
