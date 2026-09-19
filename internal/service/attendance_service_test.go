package service

import (
	"testing"
	"time"

	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func attendanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.User{}, &models.Student{}, &models.AcademicSession{}, &models.Term{}, &models.SchoolClass{}, &models.Section{}, &models.StudentEnrollment{}, &models.AttendanceRecord{}); err != nil { t.Fatal(err) }
	return db
}

func TestAttendanceValidatesEnrollmentTermAndDuplicate(t *testing.T) {
	db := attendanceTestDB(t)
	user := models.User{Name:"Attendance Student",Email:"attendance@example.com",Role:"student",Active:true}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	student := models.Student{UserID:user.ID,AdmissionNumber:"ATT-001"}
	if err := db.Create(&student).Error; err != nil { t.Fatal(err) }
	session := models.AcademicSession{Name:"2026/2027"}
	if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	term := models.Term{AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2026,12,15,0,0,0,0,time.UTC)}
	if err := db.Create(&term).Error; err != nil { t.Fatal(err) }
	class := models.SchoolClass{Name:"JSS 1",Level:1}
	if err := db.Create(&class).Error; err != nil { t.Fatal(err) }
	section := models.Section{ClassID:class.ID,Name:"A"}
	if err := db.Create(&section).Error; err != nil { t.Fatal(err) }
	enrollment := models.StudentEnrollment{StudentID:student.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:section.ID}
	enrollment.Status=models.EnrollmentStatusActive
	if err := db.Create(&enrollment).Error; err != nil { t.Fatal(err) }

	svc := NewAttendanceService(repository.NewAttendanceRepository(db),db)
	date := time.Date(2026,9,19,10,30,0,0,time.Local)
	record, err := svc.Create(models.AttendanceRecord{EnrollmentID:enrollment.ID,TermID:term.ID,Date:date,Status:models.AttendancePresent})
	if err != nil { t.Fatal(err) }
	if record.Status != models.AttendancePresent { t.Fatalf("expected present, got %s", record.Status) }
	_, err = svc.Create(models.AttendanceRecord{EnrollmentID:enrollment.ID,TermID:term.ID,Date:date,Status:models.AttendanceAbsent})
	if err != ErrAttendanceDuplicate { t.Fatalf("expected duplicate error, got %v", err) }

	otherSession := models.AcademicSession{Name:"2027/2028"}
	if err := db.Create(&otherSession).Error; err != nil { t.Fatal(err) }
	otherTerm := models.Term{AcademicSessionID:otherSession.ID,Name:models.TermFirst,StartDate:time.Date(2027,1,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,4,1,0,0,0,0,time.UTC)}
	if err := db.Create(&otherTerm).Error; err != nil { t.Fatal(err) }
	_, err = svc.Create(models.AttendanceRecord{EnrollmentID:enrollment.ID,TermID:otherTerm.ID,Date:time.Date(2026,9,20,0,0,0,0,time.UTC),Status:models.AttendancePresent})
	if err != ErrAttendanceTermMismatch { t.Fatalf("expected term mismatch, got %v", err) }

	_, err = svc.Create(models.AttendanceRecord{EnrollmentID:enrollment.ID,TermID:term.ID,Date:time.Date(2026,9,21,0,0,0,0,time.UTC),Status:"unknown"})
	if err != ErrAttendanceInvalidStatus { t.Fatalf("expected invalid status, got %v", err) }
}
