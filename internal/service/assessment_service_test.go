package service

import (
    "fmt"
    "testing"
    "time"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func assessmentTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    db,err:=gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared",t.Name())),&gorm.Config{});if err!=nil{t.Fatal(err)}
    if err:=db.AutoMigrate(&models.User{},&models.Subject{},&models.AcademicSession{},&models.Term{},&models.SchoolClass{},&models.Section{},&models.TeacherAssignment{},&models.Assessment{});err!=nil{t.Fatal(err)}
    return db
}

func TestAssessmentValidatesAssignmentScoreAndDate(t *testing.T) {
    db:=assessmentTestDB(t)
    teacher:=models.User{Name:"Teacher",Email:"teacher-assessment@example.com",Role:"teacher",Active:true};if err:=db.Create(&teacher).Error;err!=nil{t.Fatal(err)}
    subject:=models.Subject{Name:"Mathematics",Code:"MATH-A",Active:true};if err:=db.Create(&subject).Error;err!=nil{t.Fatal(err)}
    session:=models.AcademicSession{Name:"2026/2027",StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,7,31,0,0,0,0,time.UTC)};if err:=db.Create(&session).Error;err!=nil{t.Fatal(err)}
    term:=models.Term{AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2026,12,15,0,0,0,0,time.UTC)};if err:=db.Create(&term).Error;err!=nil{t.Fatal(err)}
    class:=models.SchoolClass{Name:"JSS 1",Level:1};if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)}
    assignment:=models.TeacherAssignment{TeacherID:teacher.ID,SubjectID:subject.ID,AcademicSessionID:session.ID,TermID:term.ID,ClassID:class.ID,Active:true};if err:=db.Create(&assignment).Error;err!=nil{t.Fatal(err)}
    svc:=NewAssessmentService(repository.NewAssessmentRepository(db),db)
    date:=time.Date(2026,10,10,0,0,0,0,time.UTC)
    v,err:=svc.Create(models.Assessment{TeacherAssignmentID:assignment.ID,Title:"First CA",Type:"CA",MaxScore:30,Weight:20,Date:date});if err!=nil{t.Fatal(err)}
    if v.Title!="First CA"||v.MaxScore!=30{t.Fatalf("unexpected assessment: %+v",v)}
    _,err=svc.Create(models.Assessment{TeacherAssignmentID:assignment.ID,Title:"first ca",Type:"CA",MaxScore:30,Weight:20,Date:date});if err!=ErrAssessmentDuplicate{t.Fatalf("expected duplicate, got %v",err)}
    _,err=svc.Create(models.Assessment{TeacherAssignmentID:assignment.ID,Title:"Invalid Weight",Type:"exam",MaxScore:100,Weight:101,Date:date});if err!=ErrAssessmentInvalid{t.Fatalf("expected invalid weight, got %v",err)}
    _,err=svc.Create(models.Assessment{TeacherAssignmentID:assignment.ID,Title:"Outside Term",Type:"CA",MaxScore:30,Weight:20,Date:time.Date(2027,1,10,0,0,0,0,time.UTC)});if err!=ErrAssessmentInvalid{t.Fatalf("expected invalid date, got %v",err)}
}
