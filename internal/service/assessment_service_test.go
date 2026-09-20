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

func assessmentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.Subject{}, &models.AcademicSession{}, &models.Term{}, &models.SchoolClass{}, &models.Section{}, &models.TeacherAssignment{}, &models.Assessment{}); err != nil { t.Fatal(err) }
	return db
}

func TestAssessmentValidatesAssignmentScoreAndDate(t *testing.T) {
	db := assessmentTestDB(t)
	school := models.School{Name: "School A", Code: "ASSESS-A", Status: models.SchoolStatusActive}
	if err := db.Create(&school).Error; err != nil { t.Fatal(err) }

	teacher := models.User{Name: "Teacher", Email: "teacher-assessment@example.com", Role: "teacher", Active: true, SchoolID: &school.ID}
	if err := db.Create(&teacher).Error; err != nil { t.Fatal(err) }
	subject := models.Subject{Name: "Mathematics", Code: "MATH-A", Active: true, SchoolID: school.ID}
	if err := db.Create(&subject).Error; err != nil { t.Fatal(err) }
	session := models.AcademicSession{Name: "2026/2027", StartDate: time.Date(2026,9,1,0,0,0,0,time.UTC), EndDate: time.Date(2027,7,31,0,0,0,0,time.UTC), SchoolID: school.ID}
	if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	term := models.Term{AcademicSessionID: session.ID, Name: models.TermFirst, StartDate: time.Date(2026,9,1,0,0,0,0,time.UTC), EndDate: time.Date(2026,12,15,0,0,0,0,time.UTC), SchoolID: school.ID}
	if err := db.Create(&term).Error; err != nil { t.Fatal(err) }
	class := models.SchoolClass{Name: "JSS 1", Level: 1, SchoolID: school.ID}
	if err := db.Create(&class).Error; err != nil { t.Fatal(err) }
	assignment := models.TeacherAssignment{TeacherID: teacher.ID, SubjectID: subject.ID, AcademicSessionID: session.ID, TermID: term.ID, ClassID: class.ID, Active: true, SchoolID: school.ID}
	if err := db.Create(&assignment).Error; err != nil { t.Fatal(err) }

	svc := NewAssessmentService(repository.NewAssessmentRepository(db), db)
	date := time.Date(2026,10,10,0,0,0,0,time.UTC)
	v, err := svc.Create(school.ID, models.Assessment{TeacherAssignmentID: assignment.ID, Title: "First CA", Type: "CA", MaxScore: 30, Weight: 20, Date: date})
	if err != nil { t.Fatal(err) }
	if v.Title != "First CA" || v.MaxScore != 30 || v.SchoolID != school.ID { t.Fatalf("unexpected assessment: %+v", v) }

	_, err = svc.Create(school.ID, models.Assessment{TeacherAssignmentID: assignment.ID, Title: "first ca", Type: "CA", MaxScore: 30, Weight: 20, Date: date})
	if err != ErrAssessmentDuplicate { t.Fatalf("expected duplicate, got %v", err) }
	_, err = svc.Create(school.ID, models.Assessment{TeacherAssignmentID: assignment.ID, Title: "Invalid Weight", Type: "exam", MaxScore: 100, Weight: 101, Date: date})
	if err != ErrAssessmentInvalid { t.Fatalf("expected invalid weight, got %v", err) }
	_, err = svc.Create(school.ID, models.Assessment{TeacherAssignmentID: assignment.ID, Title: "Outside Term", Type: "CA", MaxScore: 30, Weight: 20, Date: time.Date(2027,1,10,0,0,0,0,time.UTC)})
	if err != ErrAssessmentInvalid { t.Fatalf("expected invalid date, got %v", err) }
}

func TestAssessmentSchoolIsolation(t *testing.T) {
	db := assessmentTestDB(t)
	schoolA := models.School{Name: "School A", Code: "ASSESS-ISO-A", Status: models.SchoolStatusActive}
	schoolB := models.School{Name: "School B", Code: "ASSESS-ISO-B", Status: models.SchoolStatusActive}
	if err := db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&schoolB).Error; err != nil { t.Fatal(err) }

	teacher := models.User{Name: "Teacher A", Email: "teacher-iso@example.com", Role: "teacher", Active: true, SchoolID: &schoolA.ID}
	subject := models.Subject{Name: "Mathematics", Code: "MATH-ISO", Active: true, SchoolID: schoolA.ID}
	session := models.AcademicSession{Name: "2026/2027", SchoolID: schoolA.ID, StartDate: time.Date(2026,9,1,0,0,0,0,time.UTC), EndDate: time.Date(2027,7,31,0,0,0,0,time.UTC)}
	term := models.Term{Name: models.TermFirst, SchoolID: schoolA.ID, AcademicSessionID: session.ID, StartDate: time.Date(2026,9,1,0,0,0,0,time.UTC), EndDate: time.Date(2026,12,15,0,0,0,0,time.UTC)}
	class := models.SchoolClass{Name: "JSS 1", SchoolID: schoolA.ID}
	for _, v := range []interface{}{&teacher, &subject, &session, &class} { if err := db.Create(v).Error; err != nil { t.Fatal(err) } }
	term.AcademicSessionID = session.ID
	if err := db.Create(&term).Error; err != nil { t.Fatal(err) }
	assignment := models.TeacherAssignment{TeacherID: teacher.ID, SubjectID: subject.ID, AcademicSessionID: session.ID, TermID: term.ID, ClassID: class.ID, Active: true, SchoolID: schoolA.ID}
	if err := db.Create(&assignment).Error; err != nil { t.Fatal(err) }

	svc := NewAssessmentService(repository.NewAssessmentRepository(db), db)
	assessment, err := svc.Create(schoolA.ID, models.Assessment{TeacherAssignmentID: assignment.ID, Title: "CA", Type: "ca", MaxScore: 100, Weight: 100, Date: time.Date(2026,10,1,0,0,0,0,time.UTC)})
	if err != nil { t.Fatal(err) }

	if _, err := svc.Get(schoolB.ID, assessment.ID); err != ErrAssessmentNotFound { t.Fatalf("expected school B GET to be isolated, got %v", err) }
	v := assessment
	v.Title = "Changed"
	if err := svc.Update(schoolB.ID, v); err != ErrAssessmentNotFound { t.Fatalf("expected school B UPDATE to be isolated, got %v", err) }
	if err := svc.Delete(schoolB.ID, assessment.ID); err != ErrAssessmentNotFound { t.Fatalf("expected school B DELETE to be isolated, got %v", err) }
	var persisted models.Assessment
	if err := db.First(&persisted, "id = ?", assessment.ID).Error; err != nil { t.Fatal(err) }
	if persisted.Title != "CA" || persisted.SchoolID != schoolA.ID { t.Fatalf("school A assessment was changed: %+v", persisted) }
	_ = uuid.Nil
}
