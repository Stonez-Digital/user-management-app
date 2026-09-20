package service

import (
 "errors"
 "sort"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "github.com/onoja217/users-management-app/internal/repository"
 "gorm.io/gorm"
)
var(ErrResultNotFound=errors.New("assessment result not found");ErrResultAssessmentMissing=errors.New("assessment not found");ErrResultEnrollmentMissing=errors.New("student enrollment not found");ErrResultInvalid=errors.New("invalid assessment result");ErrResultDuplicate=errors.New("result already exists for this assessment and enrollment"))
type AssessmentResultService struct{repo repository.AssessmentResultRepository;db *gorm.DB}
func NewAssessmentResultService(r repository.AssessmentResultRepository,db *gorm.DB)*AssessmentResultService{return &AssessmentResultService{repo:r,db:db}}
func(s *AssessmentResultService)DB()*gorm.DB{return s.db}
func(s *AssessmentResultService)validate(schoolID uuid.UUID,v models.AssessmentResult)error{
 var assessment models.Assessment;if e:=s.db.Preload("TeacherAssignment").Where("id = ? AND school_id = ?",v.AssessmentID,schoolID).First(&assessment).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return ErrResultAssessmentMissing};return e}
 var enrollment models.StudentEnrollment;if e:=s.db.Where("id = ? AND school_id = ?",v.StudentEnrollmentID,schoolID).First(&enrollment).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return ErrResultEnrollmentMissing};return e}
 if !assessment.TeacherAssignment.Active||enrollment.Status==models.EnrollmentStatusWithdrawn||enrollment.Status==models.EnrollmentStatusCompleted{return ErrResultInvalid}
 if assessment.TeacherAssignment.AcademicSessionID!=enrollment.AcademicSessionID||assessment.TeacherAssignment.ClassID!=enrollment.ClassID{return ErrResultInvalid}
 if v.Score<0||v.Score>assessment.MaxScore{return ErrResultInvalid};return nil
}
func(s *AssessmentResultService)Create(schoolID uuid.UUID,v models.AssessmentResult)(models.AssessmentResult,error){v.SchoolID=schoolID;if e:=s.validate(schoolID,v);e!=nil{return v,e};var x models.AssessmentResult;e:=s.db.Where("school_id = ? AND assessment_id = ? AND student_enrollment_id = ?",schoolID,v.AssessmentID,v.StudentEnrollmentID).First(&x).Error;if e==nil{return v,ErrResultDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return v,e};return s.repo.Create(schoolID,v)}
func(s *AssessmentResultService)List(schoolID uuid.UUID)([]models.AssessmentResult,error){return s.repo.List(schoolID)}
func(s *AssessmentResultService)Get(schoolID,id uuid.UUID)(models.AssessmentResult,error){v,e:=s.repo.Get(schoolID,id);if errors.Is(e,gorm.ErrRecordNotFound){return v,ErrResultNotFound};return v,e}
func(s *AssessmentResultService)Update(schoolID uuid.UUID,v models.AssessmentResult)error{if _,e:=s.Get(schoolID,v.ID);e!=nil{return e};v.SchoolID=schoolID;if e:=s.validate(schoolID,v);e!=nil{return e};var x models.AssessmentResult;e:=s.db.Where("school_id = ? AND assessment_id = ? AND student_enrollment_id = ? AND id <> ?",schoolID,v.AssessmentID,v.StudentEnrollmentID,v.ID).First(&x).Error;if e==nil{return ErrResultDuplicate};if !errors.Is(e,gorm.ErrRecordNotFound){return e};return s.repo.Update(schoolID,v)}
func(s *AssessmentResultService)Delete(schoolID,id uuid.UUID)error{if _,e:=s.Get(schoolID,id);e!=nil{return e};return s.repo.Delete(schoolID,id)}
type ReportSubject struct{SubjectID uuid.UUID \`json:"subject_id"\`;SubjectName string \`json:"subject_name"\`;AssessmentCount int \`json:"assessment_count"\`;TotalScore float64 \`json:"total_score"\`;TotalMaxScore float64 \`json:"total_max_score"\`;Percentage float64 \`json:"percentage"\`;WeightedContribution float64 \`json:"weighted_contribution"\`}
type ReportCard struct{StudentEnrollmentID uuid.UUID \`json:"student_enrollment_id"\`;StudentID uuid.UUID \`json:"student_id"\`;TermID uuid.UUID \`json:"term_id"\`;Subjects []ReportSubject \`json:"subjects"\`;OverallPercentage float64 \`json:"overall_percentage"\`;TotalWeightedContribution float64 \`json:"total_weighted_contribution"\`}
func(s *AssessmentResultService)ReportCard(schoolID,enrollmentID,termID uuid.UUID)(ReportCard,error){
 var enrollment models.StudentEnrollment;if e:=s.db.Where("id = ? AND school_id = ?",enrollmentID,schoolID).First(&enrollment).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return ReportCard{},ErrResultEnrollmentMissing};return ReportCard{},e}
 var term models.Term;if e:=s.db.Where("id = ? AND school_id = ? AND academic_session_id = ?",termID,schoolID,enrollment.AcademicSessionID).First(&term).Error;e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return ReportCard{},ErrResultInvalid};return ReportCard{},e}
 var results []models.AssessmentResult;if e:=s.db.Where("school_id = ? AND student_enrollment_id = ?",schoolID,enrollmentID).Preload("Assessment","school_id = ?",schoolID).Preload("Assessment.TeacherAssignment","school_id = ?",schoolID).Preload("Assessment.TeacherAssignment.Subject","school_id = ?",schoolID).Find(&results).Error;e!=nil{return ReportCard{},e}
 groups:=map[uuid.UUID]*ReportSubject{};var totalScore,totalMax,weighted,totalWeight float64
 for _,result:=range results{assignment:=result.Assessment.TeacherAssignment;if assignment.TermID!=term.ID{continue};subject:=assignment.Subject;g:=groups[subject.ID];if g==nil{g=&ReportSubject{SubjectID:subject.ID,SubjectName:subject.Name};groups[subject.ID]=g};g.AssessmentCount++;g.TotalScore+=result.Score;g.TotalMaxScore+=result.Assessment.MaxScore;g.WeightedContribution+=(result.Score/result.Assessment.MaxScore)*result.Assessment.Weight;totalScore+=result.Score;totalMax+=result.Assessment.MaxScore;weighted+=g.WeightedContribution;totalWeight+=result.Assessment.Weight}
 subjects:=make([]ReportSubject,0,len(groups));for _,g:=range groups{if g.TotalMaxScore>0{g.Percentage=(g.TotalScore/g.TotalMaxScore)*100};subjects=append(subjects,*g)};sort.Slice(subjects,func(i,j int)bool{return subjects[i].SubjectName<subjects[j].SubjectName});overall:=0.0;if totalWeight>0{overall=(weighted/totalWeight)*100}else if totalMax>0{overall=(totalScore/totalMax)*100}
 return ReportCard{StudentEnrollmentID:enrollment.ID,StudentID:enrollment.StudentID,TermID:term.ID,Subjects:subjects,OverallPercentage:overall,TotalWeightedContribution:weighted},nil
}
