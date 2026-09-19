package service

import("errors";"strings";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")
var(ErrSubjectNotFound=errors.New("subject not found");ErrSubjectDuplicate=errors.New("subject already exists");ErrSubjectInvalid=errors.New("subject code and name are required"))
type SubjectService struct{repo repository.SubjectRepository;db *gorm.DB}
func NewSubjectService(repo repository.SubjectRepository,db *gorm.DB)*SubjectService{return &SubjectService{repo,db}}
func(s *SubjectService)DB()*gorm.DB{return s.db}
func(s *SubjectService)norm(v models.Subject)(models.Subject,error){v.Code=strings.ToUpper(strings.TrimSpace(v.Code));v.Name=strings.TrimSpace(v.Name);v.Description=strings.TrimSpace(v.Description);if v.Code==""||v.Name==""{return v,ErrSubjectInvalid};return v,nil}
func(s *SubjectService)Create(v models.Subject)(models.Subject,error){var e error;if v,e=s.norm(v);e!=nil{return v,e};var x models.Subject;if e=s.db.Where("code = ? OR name = ?",v.Code,v.Name).First(&x).Error;e==nil{return v,ErrSubjectDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};return s.repo.Create(v)}
func(s *SubjectService)List()([]models.Subject,error){return s.repo.List()}
func(s *SubjectService)Get(id uuid.UUID)(models.Subject,error){v,e:=s.repo.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrSubjectNotFound};return v,e}
func(s *SubjectService)Update(v models.Subject)error{if _,e:=s.Get(v.ID);e!=nil{return e};var e error;if v,e=s.norm(v);e!=nil{return e};var x models.Subject;if e=s.db.Where("(code = ? OR name = ?) AND id <> ?",v.Code,v.Name,v.ID).First(&x).Error;e==nil{return ErrSubjectDuplicate}else if !errors.Is(e,gorm.ErrRecordNotFound){return e};return s.repo.Update(v)}
func(s *SubjectService)Delete(id uuid.UUID)error{if _,e:=s.Get(id);e!=nil{return e};return s.repo.Delete(id)}
