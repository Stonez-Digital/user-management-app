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


func TestEnrollmentHistoryIsSchoolScoped(t *testing.T) {
    db := enrollmentTestDB(t)
    schoolID := uuid.New()
    otherSchoolID := uuid.New()
    user := models.User{SchoolID:&schoolID,Name:"History Student",Email:"history-"+schoolID.String()+"@example.com",Role:"student",Active:true}
    if err:=db.Create(&user).Error;err!=nil{t.Fatal(err)}
    student := models.Student{SchoolID:schoolID,UserID:user.ID,AdmissionNumber:"H-001"}
    if err:=db.Create(&student).Error;err!=nil{t.Fatal(err)}
    otherUser := models.User{SchoolID:&otherSchoolID,Name:"Other Student",Email:"other-"+otherSchoolID.String()+"@example.com",Role:"student",Active:true}
    if err:=db.Create(&otherUser).Error;err!=nil{t.Fatal(err)}
    otherStudent := models.Student{SchoolID:otherSchoolID,UserID:otherUser.ID,AdmissionNumber:"H-002"}
    if err:=db.Create(&otherStudent).Error;err!=nil{t.Fatal(err)}
    session := models.AcademicSession{SchoolID:schoolID,Name:"2030/2031"}
    if err:=db.Create(&session).Error;err!=nil{t.Fatal(err)}
    class := models.SchoolClass{SchoolID:schoolID,Name:"JSS 2",Level:2}
    if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)}
    section := models.Section{SchoolID:schoolID,ClassID:class.ID,Name:"A"}
    if err:=db.Create(&section).Error;err!=nil{t.Fatal(err)}
    svc:=NewEnrollmentService(repository.NewEnrollmentRepository(db),db)
    if _,err:=svc.Create(schoolID,models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:section.ID});err!=nil{t.Fatal(err)}
    history,err:=svc.History(schoolID,student.ID)
    if err!=nil{t.Fatal(err)}
    if len(history)!=1 || history[0].StudentID!=student.ID{t.Fatalf("unexpected history: %#v",history)}
    if _,err:=svc.History(schoolID,otherStudent.ID);err!=ErrEnrollmentStudentMissing{t.Fatalf("expected school-scoped student lookup to fail, got %v",err)}
}


func TestEnrollmentPromotionPreservesHistoryAndCompletesSource(t *testing.T) {
	db := enrollmentTestDB(t)
	schoolID := uuid.New()
	user := models.User{SchoolID:&schoolID,Name:"Promotion Student",Email:"promotion-"+schoolID.String()+"@example.com",Role:"student",Active:true}
	if err:=db.Create(&user).Error;err!=nil{t.Fatal(err)}
	student:=models.Student{SchoolID:schoolID,UserID:user.ID,AdmissionNumber:"PROM-001"};if err:=db.Create(&student).Error;err!=nil{t.Fatal(err)}
	session1:=models.AcademicSession{SchoolID:schoolID,Name:"2030/2031"};session2:=models.AcademicSession{SchoolID:schoolID,Name:"2031/2032"}
	if err:=db.Create(&session1).Error;err!=nil{t.Fatal(err)};if err:=db.Create(&session2).Error;err!=nil{t.Fatal(err)}
	class1:=models.SchoolClass{SchoolID:schoolID,Name:"JSS 1",Level:1};class2:=models.SchoolClass{SchoolID:schoolID,Name:"JSS 2",Level:2}
	if err:=db.Create(&class1).Error;err!=nil{t.Fatal(err)};if err:=db.Create(&class2).Error;err!=nil{t.Fatal(err)}
	sec1:=models.Section{SchoolID:schoolID,ClassID:class1.ID,Name:"A"};sec2:=models.Section{SchoolID:schoolID,ClassID:class2.ID,Name:"A"}
	if err:=db.Create(&sec1).Error;err!=nil{t.Fatal(err)};if err:=db.Create(&sec2).Error;err!=nil{t.Fatal(err)}
	svc:=NewEnrollmentService(repository.NewEnrollmentRepository(db),db)
	source,err:=svc.Create(schoolID,models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session1.ID,ClassID:class1.ID,SectionID:sec1.ID});if err!=nil{t.Fatal(err)}
	target,err:=svc.Place(schoolID,source.ID,EnrollmentPlacementRequest{TargetSessionID:session2.ID,TargetClassID:class2.ID,TargetSectionID:sec2.ID,Operation:"promote"});if err!=nil{t.Fatal(err)}
	if target.ID==source.ID||target.AcademicSessionID!=session2.ID||target.ClassID!=class2.ID{t.Fatalf("unexpected promoted enrollment: %#v",target)}
	updated,err:=svc.Get(schoolID,source.ID);if err!=nil{t.Fatal(err)}
	if updated.Status!=models.EnrollmentStatusCompleted{t.Fatalf("expected source enrollment completed, got %s",updated.Status)}
	history,err:=svc.History(schoolID,student.ID);if err!=nil{t.Fatal(err)}
	if len(history)!=2{t.Fatalf("expected two historical enrollments, got %d",len(history))}
}

func TestEnrollmentReenrollmentRequiresNonActiveSource(t *testing.T) {
	db:=enrollmentTestDB(t);schoolID:=uuid.New()
	user:=models.User{SchoolID:&schoolID,Name:"Reenroll Student",Email:"reenroll-"+schoolID.String()+"@example.com",Role:"student",Active:true};if err:=db.Create(&user).Error;err!=nil{t.Fatal(err)}
	student:=models.Student{SchoolID:schoolID,UserID:user.ID,AdmissionNumber:"RE-001"};if err:=db.Create(&student).Error;err!=nil{t.Fatal(err)}
	session1:=models.AcademicSession{SchoolID:schoolID,Name:"2032/2033"};session2:=models.AcademicSession{SchoolID:schoolID,Name:"2033/2034"};if err:=db.Create(&session1).Error;err!=nil{t.Fatal(err)};if err:=db.Create(&session2).Error;err!=nil{t.Fatal(err)}
	class:=models.SchoolClass{SchoolID:schoolID,Name:"SS 1",Level:4};if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)};sec:=models.Section{SchoolID:schoolID,ClassID:class.ID,Name:"A"};if err:=db.Create(&sec).Error;err!=nil{t.Fatal(err)}
	svc:=NewEnrollmentService(repository.NewEnrollmentRepository(db),db)
	source,err:=svc.Create(schoolID,models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session1.ID,ClassID:class.ID,SectionID:sec.ID});if err!=nil{t.Fatal(err)}
	if _,err:=svc.Place(schoolID,source.ID,EnrollmentPlacementRequest{TargetSessionID:session2.ID,TargetClassID:class.ID,TargetSectionID:sec.ID,Operation:"reenroll"});err!=ErrEnrollmentPromotionSource{t.Fatalf("expected active source rejection, got %v",err)}
	if err:=svc.Update(schoolID,models.StudentEnrollment{ID:source.ID,StudentID:source.StudentID,AcademicSessionID:source.AcademicSessionID,ClassID:source.ClassID,SectionID:source.SectionID,Status:models.EnrollmentStatusWithdrawn});err!=nil{t.Fatal(err)}
	target,err:=svc.Place(schoolID,source.ID,EnrollmentPlacementRequest{TargetSessionID:session2.ID,TargetClassID:class.ID,TargetSectionID:sec.ID,Operation:"reenroll"});if err!=nil{t.Fatal(err)}
	if target.Status!=models.EnrollmentStatusActive||target.AcademicSessionID!=session2.ID{t.Fatalf("unexpected reenrollment: %#v",target)}
}
