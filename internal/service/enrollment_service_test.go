package service

import (
	"fmt"
	"testing"
    "github.com/google/uuid"
    "time"

	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func enrollmentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.User{}, &models.Student{}, &models.AcademicSession{}, &models.Term{}, &models.SchoolClass{}, &models.Section{}, &models.StudentEnrollment{}); err != nil { t.Fatal(err) }
	return db
}

func TestCreateEnrollmentValidatesRelationshipsAndDuplicate(t *testing.T) {
	db := enrollmentTestDB(t)
	schoolID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	user := models.User{SchoolID:&schoolID,Name:"Test Student",Email:"student@example.com",Role:"student",Active:true}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	student := models.Student{SchoolID:schoolID,UserID:user.ID,AdmissionNumber:"ADM-001"}
	if err := db.Create(&student).Error; err != nil { t.Fatal(err) }
	session := models.AcademicSession{SchoolID:schoolID,Name:"2026/2027"}
	if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	class := models.SchoolClass{SchoolID:schoolID,Name:"JSS 1",Level:1}
	if err := db.Create(&class).Error; err != nil { t.Fatal(err) }
	section := models.Section{SchoolID:schoolID,ClassID:class.ID,Name:"A"}
	if err := db.Create(&section).Error; err != nil { t.Fatal(err) }

	svc := NewEnrollmentService(repository.NewEnrollmentRepository(db),db)
	enrollment, err := svc.Create(schoolID,models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:section.ID})
	if err != nil { t.Fatal(err) }
	if enrollment.Status != models.EnrollmentStatusActive { t.Fatalf("expected active status, got %s", enrollment.Status) }

	_, err = svc.Create(schoolID,models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:section.ID})
	if err != ErrEnrollmentDuplicate { t.Fatalf("expected duplicate error, got %v", err) }

	otherClass := models.SchoolClass{SchoolID:schoolID,Name:"JSS 2",Level:2}
	if err := db.Create(&otherClass).Error; err != nil { t.Fatal(err) }
	_, err = svc.Create(schoolID,models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session.ID,ClassID:otherClass.ID,SectionID:section.ID})
	if err != ErrEnrollmentSectionMismatch { t.Fatalf("expected section/class mismatch, got %v", err) }
}


func TestEnrollmentDeleteProtectsAcademicHistory(t *testing.T) {
    db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.Student{}, &models.AcademicSession{}, &models.Term{}, &models.SchoolClass{}, &models.Section{}, &models.StudentEnrollment{}, &models.AttendanceRecord{}, &models.Assessment{}, &models.AssessmentResult{}, &models.Invoice{}); err != nil { t.Fatal(err) }
    schoolID:=uuid.New()
    school:=models.School{ID:schoolID,Name:"History School",Code:"EN-H",Status:models.SchoolStatusActive}; if err:=db.Create(&school).Error;err!=nil{t.Fatal(err)}
    user:=models.User{SchoolID:&schoolID,Name:"History Student",Email:"history@example.com",Role:"student",Active:true};if err:=db.Create(&user).Error;err!=nil{t.Fatal(err)}
    student:=models.Student{SchoolID:schoolID,UserID:user.ID,AdmissionNumber:"EN-H-001"};if err:=db.Create(&student).Error;err!=nil{t.Fatal(err)}
    session:=models.AcademicSession{SchoolID:schoolID,Name:"2029/2030"};if err:=db.Create(&session).Error;err!=nil{t.Fatal(err)}
    term:=models.Term{SchoolID:schoolID,AcademicSessionID:session.ID,Name:models.TermFirst};if err:=db.Create(&term).Error;err!=nil{t.Fatal(err)}
    class:=models.SchoolClass{SchoolID:schoolID,Name:"SS 1",Level:4};if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)}
    section:=models.Section{SchoolID:schoolID,ClassID:class.ID,Name:"A"};if err:=db.Create(&section).Error;err!=nil{t.Fatal(err)}
    svc:=NewEnrollmentService(repository.NewEnrollmentRepository(db),db)
    enrollment,err:=svc.Create(schoolID,models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:section.ID});if err!=nil{t.Fatal(err)}
    if err:=db.Create(&models.AttendanceRecord{SchoolID:schoolID,EnrollmentID:enrollment.ID,Date:time.Now().UTC(),TermID:term.ID,Status:"present"}).Error;err!=nil{t.Fatal(err)}
    if err:=svc.Delete(schoolID,enrollment.ID);err!=ErrEnrollmentInUse{t.Fatalf("expected in-use error, got %v",err)}
}
