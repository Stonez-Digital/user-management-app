package service

import ("errors";"strings";"time";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/gorm")

var(ErrAssessmentNotFound=errors.New("assessment not found");ErrAssessmentAssignmentMissing=errors.New("teacher assignment not found");ErrAssessmentInvalid=errors.New("invalid assessment configuration");ErrAssessmentDuplicate=errors.New("assessment already exists for this assignment"))
type AssessmentService struct{repo repository.AssessmentRepository;db *gorm.DB}
func NewAssessmentService(r repository.AssessmentRepository,db *gorm.DB)*AssessmentService{return &AssessmentService{repo:r,db:db}}
func(s *AssessmentService)DB()*gorm.DB{return s.db}
func(s *AssessmentService)validate(v models.Assessment)error{
 var a models.TeacherAssignment;if e:=s.db.First(&a,"id = ?",v.TeacherAssignmentID).Error;errors.Is(e,gorm.ErrRecordNotFound){return ErrAssessmentAssignmentMissing}else if e!=nil{return e}
 if !a.Active||strings.TrimSpace(v.Title)==""||strings.TrimSpace(v.Type)==""||v.MaxScore<=0||v.Weight<=0||v.Weight>100||v.Date.IsZero(){return ErrAssessmentInvalid}
 var term models.Term;if e:=s.db.First(&term,"id = ? AND academic_session_id = ?",a.TermID,a.AcademicSessionID).Error;e!=nil{return ErrAssessmentInvalid}
 d:=time.Date(v.Date.Year(),v.Date.Month(),v.Date.Day(),0,0,0,0,time.UTC);start:=time.Date(term.StartDate.Year(),term.StartDate.Month(),term.StartDate.Day(),0,0,0,0,time.UTC);end:=time.Date(term.EndDate.Year(),term.EndDate.Month(),term.EndDate.Day(),0,0,0,0,time.UTC)
 if d.Before(start)||d.After(end){return ErrAssessmentInvalid};return nil
}
func(s *AssessmentService)Create(v models.Assessment)(models.Assessment,error){if e:=s.validate(v);e!=nil{return v,e};v.Title=strings.TrimSpace(v.Title);v.Type=strings.ToLower(strings.TrimSpace(v.Type));v.Date=time.Date(v.Date.Year(),v.Date.Month(),v.Date.Day(),0,0,0,0,time.UTC);var x models.Assessment;e:=s.db.Where("teacher_assignment_id = ? AND lower(title) = lower(?)",v.TeacherAssignmentID,v.Title).First(&x).Error;if e==nil{return v,ErrAssessmentDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};return s.repo.Create(v)}
func(s *AssessmentService)List()([]models.Assessment,error){return s.repo.List()}
func(s *AssessmentService)Get(id uuid.UUID)(models.Assessment,error){v,e:=s.repo.Get(id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrAssessmentNotFound};return v,e}
func(s *AssessmentService)Update(v models.Assessment)error{if _,e:=s.Get(v.ID);e!=nil{return e};if e:=s.validate(v);e!=nil{return e};v.Title=strings.TrimSpace(v.Title);v.Type=strings.ToLower(strings.TrimSpace(v.Type));v.Date=time.Date(v.Date.Year(),v.Date.Month(),v.Date.Day(),0,0,0,0,time.UTC);var x models.Assessment;e:=s.db.Where("teacher_assignment_id = ? AND lower(title) = lower(?) AND id <> ?",v.TeacherAssignmentID,v.Title,v.ID).First(&x).Error;if e==nil{return ErrAssessmentDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return e};return s.repo.Update(v)}
func(s *AssessmentService)Delete(id uuid.UUID)error{if _,e:=s.Get(id);e!=nil{return e};return s.repo.Delete(id)}
