package service

import (
	"testing"
	"github.com/google/uuid"
	"time"

	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func resultTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.Student{}, &models.Subject{}, &models.AcademicSession{}, &models.Term{}, &models.SchoolClass{}, &models.Section{}, &models.StudentEnrollment{}, &models.TeacherAssignment{}, &models.Assessment{}, &models.AssessmentResult{}); err != nil { t.Fatal(err) }
	return db
}

func TestAssessmentResultValidationAndReportCard(t *testing.T) {
	db := resultTestDB(t)
	school := models.School{Name: "School A", Code: "RESULT-A", Status: models.SchoolStatusActive}
	if err := db.Create(&school).Error; err != nil { t.Fatal(err) }

	teacher := models.User{Name: "Teacher", Email: "teacher-result@example.com", Role: "teacher", Active: true, SchoolID: &school.ID}
	studentUser := models.User{Name: "Student", Email: "student-result@example.com", Role: "student", Active: true, SchoolID: &school.ID}
	if err := db.Create(&teacher).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&studentUser).Error; err != nil { t.Fatal(err) }
	student := models.Student{UserID: studentUser.ID, AdmissionNumber: "STU-RESULT-1", SchoolID: school.ID}
	if err := db.Create(&student).Error; err != nil { t.Fatal(err) }
	subject := models.Subject{Name: "Mathematics", Code: "MATH-R", Active: true, SchoolID: school.ID}
	if err := db.Create(&subject).Error; err != nil { t.Fatal(err) }
	session := models.AcademicSession{Name: "2026/2027", StartDate: time.Date(2026,9,1,0,0,0,0,time.UTC), EndDate: time.Date(2027,7,31,0,0,0,0,time.UTC), SchoolID: school.ID}
	if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	term := models.Term{AcademicSessionID: session.ID, Name: models.TermFirst, StartDate: time.Date(2026,9,1,0,0,0,0,time.UTC), EndDate: time.Date(2026,12,15,0,0,0,0,time.UTC), SchoolID: school.ID}
	if err := db.Create(&term).Error; err != nil { t.Fatal(err) }
	class := models.SchoolClass{Name: "JSS 1", Level: 1, SchoolID: school.ID}
	if err := db.Create(&class).Error; err != nil { t.Fatal(err) }
	section := models.Section{Name: "A", ClassID: class.ID, SchoolID: school.ID}
	if err := db.Create(&section).Error; err != nil { t.Fatal(err) }
	enrollment := models.StudentEnrollment{StudentID: student.ID, AcademicSessionID: session.ID, ClassID: class.ID, SectionID: section.ID, Status: models.EnrollmentStatusActive, EnrolledAt: time.Now().UTC(), SchoolID: school.ID}
	if err := db.Create(&enrollment).Error; err != nil { t.Fatal(err) }
	assignment := models.TeacherAssignment{TeacherID: teacher.ID, SubjectID: subject.ID, AcademicSessionID: session.ID, TermID: term.ID, ClassID: class.ID, Active: true, SchoolID: school.ID}
	if err := db.Create(&assignment).Error; err != nil { t.Fatal(err) }
	assessment1 := models.Assessment{TeacherAssignmentID: assignment.ID, Title: "CA 1", Type: "ca", MaxScore: 40, Weight: 40, Date: time.Date(2026,10,10,0,0,0,0,time.UTC), SchoolID: school.ID}
	assessment2 := models.Assessment{TeacherAssignmentID: assignment.ID, Title: "Exam", Type: "exam", MaxScore: 60, Weight: 60, Date: time.Date(2026,12,1,0,0,0,0,time.UTC), SchoolID: school.ID}
	if err := db.Create(&assessment1).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&assessment2).Error; err != nil { t.Fatal(err) }

	svc := NewAssessmentResultService(repository.NewAssessmentResultRepository(db), db)
	result, err := svc.Create(school.ID, models.AssessmentResult{AssessmentID: assessment1.ID, StudentEnrollmentID: enrollment.ID, Score: 30})
	if err != nil { t.Fatal(err) }
	if result.Score != 30 || result.SchoolID != school.ID { t.Fatalf("unexpected result: %+v", result) }
	if _, err = svc.Create(school.ID, models.AssessmentResult{AssessmentID: assessment1.ID, StudentEnrollmentID: enrollment.ID, Score: 20}); err != ErrResultDuplicate { t.Fatalf("expected duplicate, got %v", err) }
	if _, err = svc.Create(school.ID, models.AssessmentResult{AssessmentID: assessment2.ID, StudentEnrollmentID: enrollment.ID, Score: 61}); err != ErrResultInvalid { t.Fatalf("expected max-score validation, got %v", err) }
	if _, err = svc.Create(school.ID, models.AssessmentResult{AssessmentID: assessment2.ID, StudentEnrollmentID: enrollment.ID, Score: 45}); err != nil { t.Fatal(err) }
	report, err := svc.ReportCardForSchool(school.ID, enrollment.ID, session.ID, term.ID)
	if err != nil { t.Fatal(err) }
	if len(report.Subjects) != 1 { t.Fatalf("expected one subject, got %d", len(report.Subjects)) }
	if report.Subjects[0].Percentage != 75 { t.Fatalf("expected 75%% subject percentage, got %v", report.Subjects[0].Percentage) }
	if report.OverallPercentage != 75 { t.Fatalf("expected 75%% overall percentage, got %v", report.OverallPercentage) }
	if report.TotalWeightedContribution != 75 { t.Fatalf("expected 75 weighted contribution, got %v", report.TotalWeightedContribution) }
	if _, err = svc.ReportCardForSchool(school.ID, enrollment.ID, uuid.New(), term.ID); err != ErrResultInvalid { t.Fatalf("expected session mismatch to be rejected, got %v", err) }

	assessment3 := models.Assessment{TeacherAssignmentID: assignment.ID, Title: "Project", Type: "project", MaxScore: 100, Weight: 20, Date: time.Date(2026,11,15,0,0,0,0,time.UTC), SchoolID: school.ID}
	if err := db.Create(&assessment3).Error; err != nil { t.Fatal(err) }
	if _, err = svc.Create(school.ID, models.AssessmentResult{AssessmentID: assessment3.ID, StudentEnrollmentID: enrollment.ID, Score: 100}); err != nil { t.Fatal(err) }
	report, err = svc.ReportCardForSchool(school.ID, enrollment.ID, session.ID, term.ID)
	if err != nil { t.Fatal(err) }
	if report.OverallPercentage < 79.1666 || report.OverallPercentage > 79.1667 { t.Fatalf("expected weighted overall percentage 79.1667, got %v", report.OverallPercentage) }
}

func TestAssessmentResultSchoolIsolation(t *testing.T) {
	db := resultTestDB(t)
	schoolA := models.School{Name: "School A", Code: "RESULT-ISO-A", Status: models.SchoolStatusActive}
	schoolB := models.School{Name: "School B", Code: "RESULT-ISO-B", Status: models.SchoolStatusActive}
	if err := db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&schoolB).Error; err != nil { t.Fatal(err) }

	studentUser := models.User{Name: "Student", Email: "student-result-iso@example.com", Role: "student", Active: true, SchoolID: &schoolA.ID}
	if err := db.Create(&studentUser).Error; err != nil { t.Fatal(err) }
	student := models.Student{UserID: studentUser.ID, AdmissionNumber: "STU-ISO", SchoolID: schoolA.ID}
	if err := db.Create(&student).Error; err != nil { t.Fatal(err) }
	session := models.AcademicSession{Name: "2026/2027", SchoolID: schoolA.ID}
	if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	term := models.Term{Name: models.TermFirst, AcademicSessionID: session.ID, SchoolID: schoolA.ID, StartDate: time.Date(2026,9,1,0,0,0,0,time.UTC), EndDate: time.Date(2026,12,15,0,0,0,0,time.UTC)}
	if err := db.Create(&term).Error; err != nil { t.Fatal(err) }
	class := models.SchoolClass{Name: "JSS 1", SchoolID: schoolA.ID}
	if err := db.Create(&class).Error; err != nil { t.Fatal(err) }
	subject := models.Subject{Name: "Math", Code: "MATH-ISO", SchoolID: schoolA.ID, Active: true}
	if err := db.Create(&subject).Error; err != nil { t.Fatal(err) }
	teacher := models.User{Name: "Teacher", Email: "teacher-result-iso@example.com", Role: "teacher", Active: true, SchoolID: &schoolA.ID}
	if err := db.Create(&teacher).Error; err != nil { t.Fatal(err) }
	enrollment := models.StudentEnrollment{StudentID: student.ID, AcademicSessionID: session.ID, ClassID: class.ID, Status: models.EnrollmentStatusActive, SchoolID: schoolA.ID}
	if err := db.Create(&enrollment).Error; err != nil { t.Fatal(err) }
	assignment := models.TeacherAssignment{TeacherID: teacher.ID, SubjectID: subject.ID, AcademicSessionID: session.ID, TermID: term.ID, ClassID: class.ID, Active: true, SchoolID: schoolA.ID}
	if err := db.Create(&assignment).Error; err != nil { t.Fatal(err) }
	assessment := models.Assessment{TeacherAssignmentID: assignment.ID, Title: "CA", Type: "ca", MaxScore: 100, Weight: 100, Date: time.Date(2026,10,1,0,0,0,0,time.UTC), SchoolID: schoolA.ID}
	if err := db.Create(&assessment).Error; err != nil { t.Fatal(err) }
	svc := NewAssessmentResultService(repository.NewAssessmentResultRepository(db), db)
	result, err := svc.Create(schoolA.ID, models.AssessmentResult{AssessmentID: assessment.ID, StudentEnrollmentID: enrollment.ID, Score: 80})
	if err != nil { t.Fatal(err) }

	if _, err := svc.Get(schoolB.ID, result.ID); err != ErrResultNotFound { t.Fatalf("expected school B GET to be isolated, got %v", err) }
	v := result; v.Score = 10
	if err := svc.Update(schoolB.ID, v); err != ErrResultNotFound { t.Fatalf("expected school B UPDATE to be isolated, got %v", err) }
	if err := svc.Delete(schoolB.ID, result.ID); err != ErrResultNotFound { t.Fatalf("expected school B DELETE to be isolated, got %v", err) }
	report, err := svc.ReportCardForSchool(schoolB.ID, enrollment.ID, term.ID)
	if err != ErrResultEnrollmentMissing { t.Fatalf("expected cross-school report card rejection, got report=%+v err=%v", report, err) }
}
