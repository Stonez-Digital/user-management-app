package service

import (
    "errors"
    "strings"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/gorm"
)

var(
    ErrGuardianInvalid=errors.New("invalid guardian relationship")
    ErrGuardianForbidden=errors.New("guardian is not linked to student")
)

type GuardianService struct{repo repository.GuardianRepository;db *gorm.DB}
func NewGuardianService(r repository.GuardianRepository,db *gorm.DB)*GuardianService{return &GuardianService{repo:r,db:db}}
func(s *GuardianService)DB()*gorm.DB{return s.db}

type GuardianChild struct{
    StudentID uuid.UUID `json:"student_id"`
    Name string `json:"name"`
    AdmissionNumber string `json:"admission_number"`
    Relationship string `json:"relationship"`
    Primary bool `json:"primary"`
}
func(s *GuardianService)Children(guardianID uuid.UUID)([]GuardianChild,error){
    rels,e:=s.repo.ListByGuardian(guardianID);if e!=nil{return nil,e}
    out:=make([]GuardianChild,0,len(rels))
    for _,rel:=range rels{var st models.Student;if e=s.db.Preload("User").First(&st,"id=?",rel.StudentID).Error;e!=nil{continue};out=append(out,GuardianChild{StudentID:st.ID,Name:st.User.Name,AdmissionNumber:st.AdmissionNumber,Relationship:rel.Relationship,Primary:rel.Primary})}
    return out,nil
}
func(s *GuardianService)authorize(guardianID,studentID uuid.UUID)error{if _,e:=s.repo.Find(guardianID,studentID);e!=nil{if errors.Is(e,gorm.ErrRecordNotFound){return ErrGuardianForbidden};return e};return nil}

func(s *GuardianService)Attendance(guardianID,studentID uuid.UUID)([]models.Attendance,error){
    if e:=s.authorize(guardianID,studentID);e!=nil{return nil,e}
    var enrollment models.StudentEnrollment;if e:=s.db.Where("student_id=?",studentID).Order("created_at DESC").First(&enrollment).Error;e!=nil{return nil,ErrGuardianInvalid}
    var out []models.Attendance;e:=s.db.Where("enrollment_id=?",enrollment.ID).Order("date DESC").Find(&out).Error;return out,e
}
func(s *GuardianService)ReportCard(guardianID,studentID,termID uuid.UUID)(map[string]interface{},error){
    if e:=s.authorize(guardianID,studentID);e!=nil{return nil,e}
    var enrollment models.StudentEnrollment;if e:=s.db.Where("student_id=? AND academic_session_id=(SELECT academic_session_id FROM terms WHERE id=?)",studentID,termID).First(&enrollment).Error;e!=nil{return nil,ErrGuardianInvalid}
    var term models.Term;if e:=s.db.First(&term,"id=?",termID).Error;e!=nil{return nil,ErrGuardianInvalid}
    var assessments []models.Assessment;q:=s.db.Model(&models.TeacherAssignment{}).Select("id").Where("term_id=? AND academic_session_id=?",termID,enrollment.AcademicSessionID);if e:=s.db.Where("teacher_assignment_id IN (?)",q).Find(&assessments).Error;e!=nil{return nil,e}
    type item struct{SubjectID uuid.UUID `json:"subject_id"`;SubjectName string `json:"subject_name"`;Score float64 `json:"score"`;MaxScore float64 `json:"max_score"`;WeightedScore float64 `json:"weighted_score"`;Percentage float64 `json:"percentage"`}
    items:=make([]item,0)
    for _,a:=range assessments{var r models.AssessmentResult;if e:=s.db.Where("assessment_id=? AND enrollment_id=?",a.ID,enrollment.ID).First(&r).Error;e!=nil{continue};var ta models.TeacherAssignment;if e=s.db.First(&ta,"id=?",a.TeacherAssignmentID).Error;e!=nil{continue};var sub models.Subject;if e=s.db.First(&sub,"id=?",ta.SubjectID).Error;e!=nil{continue};found:=-1;for i:=range items{if items[i].SubjectID==sub.ID{found=i;break}};if found<0{items=append(items,item{SubjectID:sub.ID,SubjectName:sub.Name});found=len(items)-1};items[found].Score+=r.Score;items[found].MaxScore+=a.MaxScore;if a.MaxScore>0{items[found].WeightedScore+=(r.Score/a.MaxScore)*a.Weight}}
    for i:=range items{if items[i].MaxScore>0{items[i].Percentage=items[i].Score/items[i].MaxScore*100}}
    return map[string]interface{}{"student_id":studentID,"enrollment_id":enrollment.ID,"term_id":term.ID,"items":items},nil
}
func(s *GuardianService)Timetable(guardianID,studentID uuid.UUID)([]models.TimetableEntry,error){
    if e:=s.authorize(guardianID,studentID);e!=nil{return nil,e}
    var enrollment models.StudentEnrollment;if e:=s.db.Where("student_id=?",studentID).Order("created_at DESC").First(&enrollment).Error;e!=nil{return nil,ErrGuardianInvalid}
    var out []models.TimetableEntry;q:=s.db.Where("academic_session_id=? AND active=true AND class_id=?",enrollment.AcademicSessionID,enrollment.ClassID);if enrollment.SectionID!=uuid.Nil{q=q.Where("section_id IS NULL OR section_id=?",enrollment.SectionID)};e:=q.Order("day_of_week,start_time").Find(&out).Error;return out,e
}
func(s *GuardianService)Invoices(guardianID,studentID uuid.UUID)([]models.Invoice,error){
    if e:=s.authorize(guardianID,studentID);e!=nil{return nil,e}
    var enrollment models.StudentEnrollment;if e:=s.db.Where("student_id=?",studentID).Order("created_at DESC").First(&enrollment).Error;e!=nil{return nil,ErrGuardianInvalid}
    var out []models.Invoice;e:=s.db.Where("enrollment_id=?",enrollment.ID).Order("created_at DESC").Find(&out).Error;return out,e
}
func(s *GuardianService)Payments(guardianID,studentID uuid.UUID)([]models.Payment,error){
    if e:=s.authorize(guardianID,studentID);e!=nil{return nil,e}
    var enrollment models.StudentEnrollment;if e:=s.db.Where("enrollment_id IN (SELECT id FROM student_enrollments WHERE student_id=?)",studentID).Order("created_at DESC").First(&enrollment).Error;e!=nil{return nil,ErrGuardianInvalid}
    var invoices []models.Invoice;if e:=s.db.Where("enrollment_id=?",enrollment.ID).Find(&invoices).Error;e!=nil{return nil,e};ids:=make([]uuid.UUID,0,len(invoices));for _,i:=range invoices{ids=append(ids,i.ID)};var out []models.Payment;if len(ids)==0{return out,nil};e:=s.db.Where("invoice_id IN ?",ids).Order("created_at DESC").Find(&out).Error;return out,e
}
func(s *GuardianService)CreateLink(guardianID,studentID uuid.UUID,relationship string,primary bool)(models.GuardianRelationship,error){
    relationship=strings.TrimSpace(relationship);if relationship==""{return models.GuardianRelationship{},ErrGuardianInvalid}
    var u models.User;if e:=s.db.First(&u,"id=?",guardianID).Error;e!=nil||u.Role!="parent"||!u.Active{return models.GuardianRelationship{},ErrGuardianInvalid}
    var st models.Student;if e:=s.db.First(&st,"id=?",studentID).Error;e!=nil{return models.GuardianRelationship{},ErrGuardianInvalid}
    if _,e:=s.repo.Find(guardianID,studentID);e==nil{return models.GuardianRelationship{},ErrGuardianInvalid}
    return s.repo.Create(models.GuardianRelationship{GuardianUserID:guardianID,StudentID:studentID,Relationship:relationship,Primary:primary,Active:true})
}
