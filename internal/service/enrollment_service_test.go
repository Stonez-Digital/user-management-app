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


func TestEnrollmentPromotionRollsBackTargetWhenSourceCompletionFails(t *testing.T) {
	db := enrollmentTestDB(t)
	schoolID := uuid.New()
	user := models.User{SchoolID: &schoolID, Name: "Rollback Student", Email: "rollback-"+schoolID.String()+"@example.com", Role: "student", Active: true}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	student := models.Student{SchoolID: schoolID, UserID: user.ID, AdmissionNumber: "RB-001"}
	if err := db.Create(&student).Error; err != nil { t.Fatal(err) }
	session1 := models.AcademicSession{SchoolID: schoolID, Name: "2034/2035"}
	session2 := models.AcademicSession{SchoolID: schoolID, Name: "2035/2036"}
	if err := db.Create(&session1).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&session2).Error; err != nil { t.Fatal(err) }
	class1 := models.SchoolClass{SchoolID: schoolID, Name: "JSS 1", Level: 1}
	class2 := models.SchoolClass{SchoolID: schoolID, Name: "JSS 2", Level: 2}
	if err := db.Create(&class1).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&class2).Error; err != nil { t.Fatal(err) }
	sec1 := models.Section{SchoolID: schoolID, ClassID: class1.ID, Name: "A"}
	sec2 := models.Section{SchoolID: schoolID, ClassID: class2.ID, Name: "A"}
	if err := db.Create(&sec1).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&sec2).Error; err != nil { t.Fatal(err) }

	if err := db.Exec(`CREATE TRIGGER fail_student_completion BEFORE UPDATE OF status ON student_enrollments
		WHEN NEW.status = 'completed'
		BEGIN SELECT RAISE(ABORT, 'forced completion failure'); END;`).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewEnrollmentService(repository.NewEnrollmentRepository(db), db)
	source, err := svc.Create(schoolID, models.StudentEnrollment{StudentID: student.ID, AcademicSessionID: session1.ID, ClassID: class1.ID, SectionID: sec1.ID})
	if err != nil { t.Fatal(err) }

	if _, err := svc.Place(schoolID, source.ID, EnrollmentPlacementRequest{
		TargetSessionID: session2.ID, TargetClassID: class2.ID, TargetSectionID: sec2.ID, Operation: "promote",
	}); err == nil {
		t.Fatal("expected promotion failure")
	}

	updated, err := svc.Get(schoolID, source.ID)
	if err != nil { t.Fatal(err) }
	if updated.Status != models.EnrollmentStatusActive {
		t.Fatalf("expected source enrollment to remain active after rollback, got %s", updated.Status)
	}

	var targetCount int64
	if err := db.Model(&models.StudentEnrollment{}).
		Where("school_id = ? AND student_id = ? AND academic_session_id = ?", schoolID, student.ID, session2.ID).
		Count(&targetCount).Error; err != nil {
		t.Fatal(err)
	}
	if targetCount != 0 {
		t.Fatalf("expected target enrollment to be rolled back, found %d", targetCount)
	}
}

func TestEnrollmentPromotionRejectsCrossSchoolTarget(t *testing.T) {
	db := enrollmentTestDB(t)
	schoolID := uuid.New()
	otherSchoolID := uuid.New()
	user := models.User{SchoolID: &schoolID, Name: "Cross School Student", Email: "cross-"+schoolID.String()+"@example.com", Role: "student", Active: true}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	student := models.Student{SchoolID: schoolID, UserID: user.ID, AdmissionNumber: "CROSS-001"}
	if err := db.Create(&student).Error; err != nil { t.Fatal(err) }

	session1 := models.AcademicSession{SchoolID: schoolID, Name: "2036/2037"}
	if err := db.Create(&session1).Error; err != nil { t.Fatal(err) }
	class1 := models.SchoolClass{SchoolID: schoolID, Name: "JSS 1", Level: 1}
	if err := db.Create(&class1).Error; err != nil { t.Fatal(err) }
	sec1 := models.Section{SchoolID: schoolID, ClassID: class1.ID, Name: "A"}
	if err := db.Create(&sec1).Error; err != nil { t.Fatal(err) }

	otherSession := models.AcademicSession{SchoolID: otherSchoolID, Name: "2037/2038"}
	if err := db.Create(&otherSession).Error; err != nil { t.Fatal(err) }
	otherClass := models.SchoolClass{SchoolID: otherSchoolID, Name: "JSS 2", Level: 2}
	if err := db.Create(&otherClass).Error; err != nil { t.Fatal(err) }
	otherSection := models.Section{SchoolID: otherSchoolID, ClassID: otherClass.ID, Name: "A"}
	if err := db.Create(&otherSection).Error; err != nil { t.Fatal(err) }

	svc := NewEnrollmentService(repository.NewEnrollmentRepository(db), db)
	source, err := svc.Create(schoolID, models.StudentEnrollment{StudentID: student.ID, AcademicSessionID: session1.ID, ClassID: class1.ID, SectionID: sec1.ID})
	if err != nil { t.Fatal(err) }

	_, err = svc.Place(schoolID, source.ID, EnrollmentPlacementRequest{
		TargetSessionID: otherSession.ID, TargetClassID: otherClass.ID, TargetSectionID: otherSection.ID, Operation: "promote",
	})
	if err != ErrEnrollmentSessionMissing {
		t.Fatalf("expected cross-school session rejection, got %v", err)
	}
}


func TestCreateEnrollmentRejectsClosedSession(t *testing.T) {
	db := enrollmentTestDB(t)
	schoolID := uuid.New()
	user := models.User{SchoolID:&schoolID,Name:"Closed Session Student",Email:"closed-"+schoolID.String()+"@example.com",Role:"student",Active:true}
	if err:=db.Create(&user).Error;err!=nil{t.Fatal(err)}
	student:=models.Student{SchoolID:schoolID,UserID:user.ID,AdmissionNumber:"CLOSED-001"};if err:=db.Create(&student).Error;err!=nil{t.Fatal(err)}
	session:=models.AcademicSession{SchoolID:schoolID,Name:"2040/2041",Status:models.AcademicStatusClosed};if err:=db.Create(&session).Error;err!=nil{t.Fatal(err)}
	class:=models.SchoolClass{SchoolID:schoolID,Name:"JSS 1",Level:1};if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)}
	section:=models.Section{SchoolID:schoolID,ClassID:class.ID,Name:"A"};if err:=db.Create(&section).Error;err!=nil{t.Fatal(err)}
	svc:=NewEnrollmentService(repository.NewEnrollmentRepository(db),db)
	if _,err:=svc.Create(schoolID,models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:section.ID});err!=ErrEnrollmentSessionUnavailable{t.Fatalf("expected unavailable session error, got %v",err)}
}

func TestCreateEnrollmentAllowsOnlyOneActiveEnrollmentAcrossSessions(t *testing.T) {
	db := enrollmentTestDB(t)
	schoolID := uuid.New()
	user := models.User{SchoolID:&schoolID,Name:"Active Enrollment Student",Email:"active-"+schoolID.String()+"@example.com",Role:"student",Active:true};if err:=db.Create(&user).Error;err!=nil{t.Fatal(err)}
	student:=models.Student{SchoolID:schoolID,UserID:user.ID,AdmissionNumber:"ACTIVE-001"};if err:=db.Create(&student).Error;err!=nil{t.Fatal(err)}
	session1:=models.AcademicSession{SchoolID:schoolID,Name:"2041/2042"};session2:=models.AcademicSession{SchoolID:schoolID,Name:"2042/2043"};if err:=db.Create(&session1).Error;err!=nil{t.Fatal(err)};if err:=db.Create(&session2).Error;err!=nil{t.Fatal(err)}
	class:=models.SchoolClass{SchoolID:schoolID,Name:"JSS 1",Level:1};if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)}
	section:=models.Section{SchoolID:schoolID,ClassID:class.ID,Name:"A"};if err:=db.Create(&section).Error;err!=nil{t.Fatal(err)}
	svc:=NewEnrollmentService(repository.NewEnrollmentRepository(db),db)
	if _,err:=svc.Create(schoolID,models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session1.ID,ClassID:class.ID,SectionID:section.ID});err!=nil{t.Fatal(err)}
	if _,err:=svc.Create(schoolID,models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session2.ID,ClassID:class.ID,SectionID:section.ID});err!=ErrEnrollmentActiveDuplicate{t.Fatalf("expected active enrollment conflict, got %v",err)}
}


func TestStudentLifecyclePromotionPreservesAcademicHistoryAndStartsNewEnrollment(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(
		&models.School{}, &models.User{}, &models.Student{}, &models.AcademicSession{}, &models.Term{},
		&models.SchoolClass{}, &models.Section{}, &models.StudentEnrollment{},
		&models.AttendanceRecord{}, &models.Subject{}, &models.TeacherAssignment{},
		&models.Assessment{}, &models.AssessmentResult{},
	); err != nil { t.Fatal(err) }

	schoolID := uuid.New()
	school := models.School{ID: schoolID, Name: "Lifecycle School", Code: "LIFE-141", Status: models.SchoolStatusActive}
	if err := db.Create(&school).Error; err != nil { t.Fatal(err) }

	user := models.User{SchoolID: &schoolID, Name: "Lifecycle Student", Email: "lifecycle-"+schoolID.String()+"@example.com", Role: "student", Active: true}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	student := models.Student{SchoolID: schoolID, UserID: user.ID, AdmissionNumber: "LIFE-001"}
	if err := db.Create(&student).Error; err != nil { t.Fatal(err) }

	session1 := models.AcademicSession{SchoolID: schoolID, Name: "2045/2046", Status: models.AcademicStatusActive}
	session2 := models.AcademicSession{SchoolID: schoolID, Name: "2046/2047", Status: models.AcademicStatusActive}
	if err := db.Create(&session1).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&session2).Error; err != nil { t.Fatal(err) }

	term1 := models.Term{SchoolID: schoolID, AcademicSessionID: session1.ID, Name: models.TermFirst, StartDate: time.Date(2045, 9, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2045, 12, 20, 0, 0, 0, 0, time.UTC)}
	if err := db.Create(&term1).Error; err != nil { t.Fatal(err) }

	class1 := models.SchoolClass{SchoolID: schoolID, Name: "JSS 1", Level: 1}
	class2 := models.SchoolClass{SchoolID: schoolID, Name: "JSS 2", Level: 2}
	if err := db.Create(&class1).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&class2).Error; err != nil { t.Fatal(err) }
	section1 := models.Section{SchoolID: schoolID, ClassID: class1.ID, Name: "A"}
	section2 := models.Section{SchoolID: schoolID, ClassID: class2.ID, Name: "A"}
	if err := db.Create(&section1).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&section2).Error; err != nil { t.Fatal(err) }

	svc := NewEnrollmentService(repository.NewEnrollmentRepository(db), db)
	source, err := svc.Create(schoolID, models.StudentEnrollment{
		StudentID: student.ID, AcademicSessionID: session1.ID, ClassID: class1.ID, SectionID: section1.ID,
	})
	if err != nil { t.Fatal(err) }

	attendance := models.AttendanceRecord{
		SchoolID: schoolID, EnrollmentID: source.ID, TermID: term1.ID,
		Date: time.Date(2045, 10, 7, 0, 0, 0, 0, time.UTC), Status: models.AttendancePresent,
	}
	if err := db.Create(&attendance).Error; err != nil { t.Fatal(err) }

	subject := models.Subject{SchoolID: schoolID, Code: "ENG", Name: "English", Active: true}
	if err := db.Create(&subject).Error; err != nil { t.Fatal(err) }

	teacher := models.User{SchoolID: &schoolID, Name: "Lifecycle Teacher", Email: "teacher-"+schoolID.String()+"@example.com", Role: "teacher", Active: true}
	if err := db.Create(&teacher).Error; err != nil { t.Fatal(err) }

	assignment := models.TeacherAssignment{
		SchoolID: schoolID, TeacherID: teacher.ID, SubjectID: subject.ID,
		AcademicSessionID: session1.ID, TermID: term1.ID, ClassID: class1.ID,
		SectionID: &section1.ID, AllocationType: models.TeacherAllocationSubject, Active: true,
	}
	if err := db.Create(&assignment).Error; err != nil { t.Fatal(err) }

	assessment := models.Assessment{
		SchoolID: schoolID, TeacherAssignmentID: assignment.ID, Title: "First Test",
		Type: "test", MaxScore: 100, Weight: 30, Date: time.Date(2045, 10, 10, 0, 0, 0, 0, time.UTC),
	}
	if err := db.Create(&assessment).Error; err != nil { t.Fatal(err) }

	result := models.AssessmentResult{
		SchoolID: schoolID, AssessmentID: assessment.ID, StudentEnrollmentID: source.ID, Score: 82,
	}
	if err := db.Create(&result).Error; err != nil { t.Fatal(err) }

	target, err := svc.Place(schoolID, source.ID, EnrollmentPlacementRequest{
		TargetSessionID: session2.ID, TargetClassID: class2.ID, TargetSectionID: section2.ID, Operation: "promote",
	})
	if err != nil { t.Fatal(err) }
	if target.Status != models.EnrollmentStatusActive { t.Fatalf("expected promoted enrollment to be active, got %s", target.Status) }
	if target.AcademicSessionID != session2.ID || target.ClassID != class2.ID || target.SectionID != section2.ID {
		t.Fatalf("unexpected target enrollment: %#v", target)
	}

	updated, err := svc.Get(schoolID, source.ID)
	if err != nil { t.Fatal(err) }
	if updated.Status != models.EnrollmentStatusCompleted {
		t.Fatalf("expected source enrollment to be completed, got %s", updated.Status)
	}

	var historicalAttendance int64
	if err := db.Model(&models.AttendanceRecord{}).
		Where("school_id = ? AND enrollment_id = ?", schoolID, source.ID).Count(&historicalAttendance).Error; err != nil {
		t.Fatal(err)
	}
	if historicalAttendance != 1 {
		t.Fatalf("expected historical attendance to remain attached to source enrollment, got %d", historicalAttendance)
	}

	var historicalResults int64
	if err := db.Model(&models.AssessmentResult{}).
		Where("school_id = ? AND student_enrollment_id = ?", schoolID, source.ID).Count(&historicalResults).Error; err != nil {
		t.Fatal(err)
	}
	if historicalResults != 1 {
		t.Fatalf("expected historical assessment result to remain attached to source enrollment, got %d", historicalResults)
	}

	var activeCount int64
	if err := db.Model(&models.StudentEnrollment{}).
		Where("school_id = ? AND student_id = ? AND status = ?", schoolID, student.ID, models.EnrollmentStatusActive).
		Count(&activeCount).Error; err != nil {
		t.Fatal(err)
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active enrollment after promotion, got %d", activeCount)
	}

	var historyCount int64
	if err := db.Model(&models.StudentEnrollment{}).
		Where("school_id = ? AND student_id = ?", schoolID, student.ID).Count(&historyCount).Error; err != nil {
		t.Fatal(err)
	}
	if historyCount != 2 {
		t.Fatalf("expected two enrollment records after promotion, got %d", historyCount)
	}

	if _, err := svc.Place(schoolID, source.ID, EnrollmentPlacementRequest{
		TargetSessionID: session2.ID, TargetClassID: class2.ID, TargetSectionID: section2.ID, Operation: "promote",
	}); err != ErrEnrollmentPromotionSource {
		t.Fatalf("expected completed source to reject a second promotion, got %v", err)
	}
}
