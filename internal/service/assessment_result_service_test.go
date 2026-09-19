package service

import (
    "testing"
    "time"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func resultTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    db,err:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{}); if err!=nil{t.Fatal(err)}
    if err:=db.AutoMigrate(&models.User{},&models.Student{},&models.Subject{},&models.AcademicSession{},&models.Term{},&models.SchoolClass{},&models.Section{},&models.StudentEnrollment{},&models.TeacherAssignment{},&models.Assessment{},&models.AssessmentResult{});err!=nil{t.Fatal(err)}
    return db
}

func TestAssessmentResultValidationAndReportCard(t *testing.T) {
    db:=resultTestDB(t)
    teacher:=models.User{Name:"Teacher",Email:"teacher-result@example.com",Role:"teacher",Active:true};if err:=db.Create(&teacher).Error;err!=nil{t.Fatal(err)}
    studentUser:=models.User{Name:"Student",Email:"student-result@example.com",Role:"student",Active:true};if err:=db.Create(&studentUser).Error;err!=nil{t.Fatal(err)}
    student:=models.Student{UserID:studentUser.ID,AdmissionNumber:"STU-RESULT-1"};if err:=db.Create(&student).Error;err!=nil{t.Fatal(err)}
    subject:=models.Subject{Name:"Mathematics",Code:"MATH-R",Active:true};if err:=db.Create(&subject).Error;err!=nil{t.Fatal(err)}
    session:=models.AcademicSession{Name:"2026/2027",StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,7,31,0,0,0,0,time.UTC)};if err:=db.Create(&session).Error;err!=nil{t.Fatal(err)}
    term:=models.Term{AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2026,12,15,0,0,0,0,time.UTC)};if err:=db.Create(&term).Error;err!=nil{t.Fatal(err)}
    class:=models.SchoolClass{Name:"JSS 1",Level:1};if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)}
    section:=models.Section{Name:"A",ClassID:class.ID};if err:=db.Create(&section).Error;err!=nil{t.Fatal(err)}
    enrollment:=models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:section.ID,Status:models.EnrollmentStatusActive,EnrolledAt:time.Now().UTC()};if err:=db.Create(&enrollment).Error;err!=nil{t.Fatal(err)}
    assignment:=models.TeacherAssignment{TeacherID:teacher.ID,SubjectID:subject.ID,AcademicSessionID:session.ID,TermID:term.ID,ClassID:class.ID,Active:true};if err:=db.Create(&assignment).Error;err!=nil{t.Fatal(err)}
    assessment1:=models.Assessment{TeacherAssignmentID:assignment.ID,Title:"CA 1",Type:"ca",MaxScore:40,Weight:40,Date:time.Date(2026,10,10,0,0,0,0,time.UTC)};if err:=db.Create(&assessment1).Error;err!=nil{t.Fatal(err)}
    assessment2:=models.Assessment{TeacherAssignmentID:assignment.ID,Title:"Exam",Type:"exam",MaxScore:60,Weight:60,Date:time.Date(2026,12,1,0,0,0,0,time.UTC)};if err:=db.Create(&assessment2).Error;err!=nil{t.Fatal(err)}
    svc:=NewAssessmentResultService(repository.NewAssessmentResultRepository(db),db)
    result,err:=svc.Create(models.AssessmentResult{AssessmentID:assessment1.ID,StudentEnrollmentID:enrollment.ID,Score:30});if err!=nil{t.Fatal(err)}
    if result.Score!=30{t.Fatalf("unexpected score: %v",result.Score)}
    _,err=svc.Create(models.AssessmentResult{AssessmentID:assessment1.ID,StudentEnrollmentID:enrollment.ID,Score:20});if err!=ErrResultDuplicate{t.Fatalf("expected duplicate, got %v",err)}
    _,err=svc.Create(models.AssessmentResult{AssessmentID:assessment2.ID,StudentEnrollmentID:enrollment.ID,Score:61});if err!=ErrResultInvalid{t.Fatalf("expected max-score validation, got %v",err)}
    if _,err=svc.Create(models.AssessmentResult{AssessmentID:assessment2.ID,StudentEnrollmentID:enrollment.ID,Score:45});err!=nil{t.Fatal(err)}
    report,err:=svc.ReportCard(enrollment.ID,term.ID);if err!=nil{t.Fatal(err)}
    if len(report.Subjects)!=1{t.Fatalf("expected one subject, got %d",len(report.Subjects))}
    if report.Subjects[0].Percentage!=75{t.Fatalf("expected 75%% subject percentage, got %v",report.Subjects[0].Percentage)}
    if report.OverallPercentage!=75{t.Fatalf("expected 75%% overall percentage, got %v",report.OverallPercentage)}
    if report.TotalWeightedContribution!=75{t.Fatalf("expected 75 weighted contribution, got %v",report.TotalWeightedContribution)}
}
