package service

import (
 "errors"
 "strings"
 "time"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "github.com/onoja217/users-management-app/internal/repository"
 "gorm.io/gorm"
)
var(ErrAssessmentNotFound=errors.New("assessment not found");ErrAssessmentAssignmentMissing=errors.New("teacher assignment not found");ErrAssessmentInvalid=errors.New("invalid assessment configuration");ErrAssessmentDuplicate=errors.New("assessment already exists for this assignment"))
type AssessmentService struct{repo repository.AssessmentRepository;db *gorm.DB}
func NewAssessmentService(r repository.AssessmentRepository,db *gorm.DB)*AssessmentService{return &AssessmentService{repo:r,db:db}}
func(s *AssessmentService)DB()*gorm.DB{return s.db}
func(s *AssessmentService)validate(schoolID uuid.UUID,v models.Assessment)error{
 var a models.TeacherAssignment;if e:=s.db.Where("id = ? AND school_id = ?",v.TeacherAssignmentID,schoolID).First(&a).Error;errors.Is(e,gorm.ErrRecordNotFound){return ErrAssessmentAssignmentMissing}else if e!=nil{return e}
 if !a.Active||strings.TrimSpace(v.Title)==""||strings.TrimSpace(v.Type)==""||v.MaxScore<=0||v.Weight<=0||v.Weight>100||v.Date.IsZero(){return ErrAssessmentInvalid}
 var term models.Term;if e:=s.db.Where("id = ? AND school_id = ? AND academic_session_id = ?",a.TermID,schoolID,a.AcademicSessionID).First(&term).Error;e!=nil{return ErrAssessmentInvalid}
 d:=time.Date(v.Date.Year(),v.Date.Month(),v.Date.Day(),0,0,0,0,time.UTC);start:=time.Date(term.StartDate.Year(),term.StartDate.Month(),term.StartDate.Day(),0,0,0,0,time.UTC);end:=time.Date(term.EndDate.Year(),term.EndDate.Month(),term.EndDate.Day(),0,0,0,0,time.UTC)
 if d.Before(start)||d.After(end){return ErrAssessmentInvalid};return nil
}
func(s *AssessmentService)Create(schoolID uuid.UUID,v models.Assessment)(models.Assessment,error){v.SchoolID=schoolID;if e:=s.validate(schoolID,v);e!=nil{return v,e};v.Title=strings.TrimSpace(v.Title);v.Type=strings.ToLower(strings.TrimSpace(v.Type));v.Date=time.Date(v.Date.Year(),v.Date.Month(),v.Date.Day(),0,0,0,0,time.UTC);var x models.Assessment;e:=s.db.Where("school_id = ? AND teacher_assignment_id = ? AND lower(title) = lower(?)",schoolID,v.TeacherAssignmentID,v.Title).First(&x).Error;if e==nil{return v,ErrAssessmentDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};return s.repo.Create(schoolID,v)}
func(s *AssessmentService) ListForTeacher(schoolID,teacherID,sessionID,termID uuid.UUID)([]models.Assessment,error){
 var rows []models.Assessment
 return rows,s.db.Where("assessments.school_id = ? AND teacher_assignments.teacher_id = ? AND teacher_assignments.academic_session_id = ? AND teacher_assignments.term_id = ?",schoolID,teacherID,sessionID,termID).Joins("JOIN teacher_assignments ON teacher_assignments.id = assessments.teacher_assignment_id").Preload("TeacherAssignment").Find(&rows).Error
}
func(s *AssessmentService) TeacherCreate(schoolID,teacherID uuid.UUID,v models.Assessment)(models.Assessment,error){
 var a models.TeacherAssignment
 if e:=s.db.Where("id = ? AND school_id = ? AND teacher_id = ? AND active = ?",v.TeacherAssignmentID,schoolID,teacherID,true).First(&a).Error;e!=nil{return v,ErrAssessmentAssignmentMissing}
 return s.Create(schoolID,v)
}
func(s *AssessmentService)List(schoolID,sessionID,termID uuid.UUID)([]models.Assessment,error){rows,e:=s.repo.List(schoolID);if e!=nil{return nil,e};out:=make([]models.Assessment,0,len(rows));for _,r:=range rows{if r.TeacherAssignment.AcademicSessionID==sessionID&&r.TeacherAssignment.TermID==termID{out=append(out,r)}};return out,nil}
func(s *AssessmentService)Get(schoolID,id uuid.UUID)(models.Assessment,error){v,e:=s.repo.Get(schoolID,id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrAssessmentNotFound};return v,e}
func(s *AssessmentService)Update(schoolID uuid.UUID,v models.Assessment)error{if _,e:=s.Get(schoolID,v.ID);e!=nil{return e};v.SchoolID=schoolID;if e:=s.validate(schoolID,v);e!=nil{return e};v.Title=strings.TrimSpace(v.Title);v.Type=strings.ToLower(strings.TrimSpace(v.Type));v.Date=time.Date(v.Date.Year(),v.Date.Month(),v.Date.Day(),0,0,0,0,time.UTC);var x models.Assessment;e:=s.db.Where("school_id = ? AND teacher_assignment_id = ? AND lower(title) = lower(?) AND id <> ?",schoolID,v.TeacherAssignmentID,v.Title,v.ID).First(&x).Error;if e==nil{return ErrAssessmentDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return e};return s.repo.Update(schoolID,v)}
func(s *AssessmentService)Delete(schoolID,id uuid.UUID)error{if _,e:=s.Get(schoolID,id);e!=nil{return e};return s.repo.Delete(schoolID,id)}
