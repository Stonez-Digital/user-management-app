package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func teacherAssignmentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.AcademicSession{}, &models.Term{}, &models.SchoolClass{}, &models.Section{}, &models.Subject{}, &models.TeacherAssignment{}); err != nil { t.Fatal(err) }
	return db
}

func TestTeacherAssignmentSchoolIsolation(t *testing.T) {
	db := teacherAssignmentTestDB(t)
	schoolA, schoolB := uuid.New(), uuid.New()
	if err := db.Create(&models.School{ID:schoolA,Name:"School A",Code:"TA-A",Status:models.SchoolStatusActive}).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&models.School{ID:schoolB,Name:"School B",Code:"TA-B",Status:models.SchoolStatusActive}).Error; err != nil { t.Fatal(err) }

	teacher := models.User{SchoolID:&schoolA,Name:"Teacher A",Email:"teacher-a@example.com",Role:"teacher",Active:true}
	if err := db.Create(&teacher).Error; err != nil { t.Fatal(err) }
	subject := models.Subject{SchoolID:schoolA,Code:"MATH",Name:"Mathematics",Active:true}
	if err := db.Create(&subject).Error; err != nil { t.Fatal(err) }
	session := models.AcademicSession{SchoolID:schoolA,Name:"2026/2027",StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,7,31,0,0,0,0,time.UTC)}
	if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	term := models.Term{SchoolID:schoolA,AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2026,12,15,0,0,0,0,time.UTC)}
	if err := db.Create(&term).Error; err != nil { t.Fatal(err) }
	class := models.SchoolClass{SchoolID:schoolA,Name:"JSS 1",Level:1}
	if err := db.Create(&class).Error; err != nil { t.Fatal(err) }

	svc := NewTeacherAssignmentService(repository.NewTeacherAssignmentRepository(db), db)
	assignment, err := svc.Create(schoolA, models.TeacherAssignment{TeacherID:teacher.ID,SubjectID:subject.ID,AcademicSessionID:session.ID,TermID:term.ID,ClassID:class.ID,Active:true})
	if err != nil { t.Fatal(err) }

	if _, err := svc.Get(schoolB, assignment.ID); err != ErrAssignmentNotFound { t.Fatalf("cross-school get: %v", err) }
	if err := svc.Update(schoolB, models.TeacherAssignment{ID:assignment.ID,TeacherID:teacher.ID,SubjectID:subject.ID,AcademicSessionID:session.ID,TermID:term.ID,ClassID:class.ID,Active:false}); err != ErrAssignmentNotFound { t.Fatalf("cross-school update: %v", err) }
	if err := svc.Delete(schoolB, assignment.ID); err != ErrAssignmentNotFound { t.Fatalf("cross-school delete: %v", err) }

	var persisted models.TeacherAssignment
	if err := db.First(&persisted, "id = ?", assignment.ID).Error; err != nil { t.Fatal(err) }
	if persisted.SchoolID != schoolA { t.Fatalf("assignment school changed unexpectedly: %s", persisted.SchoolID) }
}


func TestTeacherAssignmentListForTeacherIsScoped(t *testing.T) {
    db := teacherAssignmentTestDB(t)
    schoolID := uuid.New()
    if err := db.Create(&models.School{ID:schoolID,Name:"School",Code:"TA-C",Status:models.SchoolStatusActive}).Error; err != nil { t.Fatal(err) }
    teacherA := models.User{SchoolID:&schoolID,Name:"Teacher A",Email:"a@example.com",Role:"teacher",Active:true}
    teacherB := models.User{SchoolID:&schoolID,Name:"Teacher B",Email:"b@example.com",Role:"teacher",Active:true}
    if err := db.Create(&teacherA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&teacherB).Error; err != nil { t.Fatal(err) }
    subject := models.Subject{SchoolID:schoolID,Code:"ENG",Name:"English",Active:true}; if err:=db.Create(&subject).Error;err!=nil{t.Fatal(err)}
    session := models.AcademicSession{SchoolID:schoolID,Name:"2026/2027",StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,7,31,0,0,0,0,time.UTC)}; if err:=db.Create(&session).Error;err!=nil{t.Fatal(err)}
    term := models.Term{SchoolID:schoolID,AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:session.StartDate,EndDate:time.Date(2026,12,15,0,0,0,0,time.UTC)}; if err:=db.Create(&term).Error;err!=nil{t.Fatal(err)}
    class := models.SchoolClass{SchoolID:schoolID,Name:"JSS 2",Level:2}; if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)}
    svc:=NewTeacherAssignmentService(repository.NewTeacherAssignmentRepository(db),db)
    a,err:=svc.Create(schoolID,models.TeacherAssignment{TeacherID:teacherA.ID,SubjectID:subject.ID,AcademicSessionID:session.ID,TermID:term.ID,ClassID:class.ID,Active:true});if err!=nil{t.Fatal(err)}
    if _,err=svc.Create(schoolID,models.TeacherAssignment{TeacherID:teacherB.ID,SubjectID:subject.ID,AcademicSessionID:session.ID,TermID:term.ID,ClassID:class.ID,Active:true});err!=nil{t.Fatal(err)}
    got,err:=svc.ListForTeacher(schoolID,teacherA.ID);if err!=nil{t.Fatal(err)}
    if len(got)!=1||got[0].ID!=a.ID||got[0].TeacherID!=teacherA.ID{t.Fatalf("teacher A received wrong assignments: %#v",got)}
}


func TestTeacherAssignmentUpdateRejectsDuplicateAllocation(t *testing.T) {
    db := teacherAssignmentTestDB(t)
    schoolID := uuid.New()
    if err := db.Create(&models.School{ID:schoolID,Name:"School",Code:"TA-D",Status:models.SchoolStatusActive}).Error; err != nil { t.Fatal(err) }
    teacher := models.User{SchoolID:&schoolID,Name:"Teacher",Email:"teacher-d@example.com",Role:"teacher",Active:true}
    if err := db.Create(&teacher).Error; err != nil { t.Fatal(err) }
    subject := models.Subject{SchoolID:schoolID,Code:"MATH-D",Name:"Math",Active:true}; if err:=db.Create(&subject).Error;err!=nil{t.Fatal(err)}
    subject2 := models.Subject{SchoolID:schoolID,Code:"ENG-D",Name:"English",Active:true}; if err:=db.Create(&subject2).Error;err!=nil{t.Fatal(err)}
    session := models.AcademicSession{SchoolID:schoolID,Name:"2027/2028",StartDate:time.Date(2027,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2028,7,31,0,0,0,0,time.UTC)}; if err:=db.Create(&session).Error;err!=nil{t.Fatal(err)}
    term := models.Term{SchoolID:schoolID,AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:session.StartDate,EndDate:time.Date(2027,12,15,0,0,0,0,time.UTC)}; if err:=db.Create(&term).Error;err!=nil{t.Fatal(err)}
    class := models.SchoolClass{SchoolID:schoolID,Name:"JSS 1 D",Level:1}; if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)}
    svc:=NewTeacherAssignmentService(repository.NewTeacherAssignmentRepository(db),db)
    first,err:=svc.Create(schoolID,models.TeacherAssignment{TeacherID:teacher.ID,SubjectID:subject.ID,AcademicSessionID:session.ID,TermID:term.ID,ClassID:class.ID,Active:true});if err!=nil{t.Fatal(err)}
    second,err:=svc.Create(schoolID,models.TeacherAssignment{TeacherID:teacher.ID,SubjectID:subject2.ID,AcademicSessionID:session.ID,TermID:term.ID,ClassID:class.ID,Active:true});if err!=nil{t.Fatal(err)}
    second.SubjectID=subject.ID
    if err:=svc.Update(schoolID,second);err!=ErrAssignmentDuplicate{t.Fatalf("expected duplicate update error, got %v",err)}
    var persisted models.TeacherAssignment
    if err:=db.First(&persisted,"id = ?",first.ID).Error;err!=nil{t.Fatal(err)}
    if persisted.SubjectID!=subject.ID{t.Fatal("original assignment changed unexpectedly")}
}

func TestTeacherAssignmentDeleteProtectsAcademicDependencies(t *testing.T) {
    db := teacherAssignmentTestDB(t)
    schoolID := uuid.New()
    if err := db.Create(&models.School{ID:schoolID,Name:"School",Code:"TA-E",Status:models.SchoolStatusActive}).Error; err != nil { t.Fatal(err) }
    teacher := models.User{SchoolID:&schoolID,Name:"Teacher",Email:"teacher-e@example.com",Role:"teacher",Active:true}; if err:=db.Create(&teacher).Error;err!=nil{t.Fatal(err)}
    subject := models.Subject{SchoolID:schoolID,Code:"SCI-E",Name:"Science",Active:true}; if err:=db.Create(&subject).Error;err!=nil{t.Fatal(err)}
    session := models.AcademicSession{SchoolID:schoolID,Name:"2028/2029",StartDate:time.Date(2028,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2029,7,31,0,0,0,0,time.UTC)}; if err:=db.Create(&session).Error;err!=nil{t.Fatal(err)}
    term := models.Term{SchoolID:schoolID,AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:session.StartDate,EndDate:time.Date(2028,12,15,0,0,0,0,time.UTC)}; if err:=db.Create(&term).Error;err!=nil{t.Fatal(err)}
    class := models.SchoolClass{SchoolID:schoolID,Name:"JSS 3 E",Level:3}; if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)}
    svc:=NewTeacherAssignmentService(repository.NewTeacherAssignmentRepository(db),db)
    assignment,err:=svc.Create(schoolID,models.TeacherAssignment{TeacherID:teacher.ID,SubjectID:subject.ID,AcademicSessionID:session.ID,TermID:term.ID,ClassID:class.ID,Active:true});if err!=nil{t.Fatal(err)}
    assessment:=models.Assessment{SchoolID:schoolID,TeacherAssignmentID:assignment.ID,Title:"First CA",Type:"test",MaxScore:100,Weight:20,Date:term.StartDate}
    if err:=db.Create(&assessment).Error;err!=nil{t.Fatal(err)}
    if err:=svc.Delete(schoolID,assignment.ID);err!=ErrAssignmentInUse{t.Fatalf("expected in-use error, got %v",err)}
}
