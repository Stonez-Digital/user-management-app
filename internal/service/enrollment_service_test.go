package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnrollmentRejectsDuplicateSession(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:enrollment-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Student{}, &models.AcademicSession{}, &models.SchoolClass{}, &models.Section{}, &models.StudentEnrollment{}); err != nil {
		t.Fatal(err)
	}

	user := models.User{Name: "Student", Email: "student@test.local"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	student := models.Student{UserID: user.ID, AdmissionNumber: "ADM-1"}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	session := models.AcademicSession{Name: "2026/2027", StartDate: time.Now(), EndDate: time.Now().AddDate(1, 0, 0)}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	class := models.SchoolClass{Name: "JSS 1", Level: 1}
	if err := db.Create(&class).Error; err != nil {
		t.Fatal(err)
	}
	section := models.Section{ClassID: class.ID, Name: "A"}
	if err := db.Create(&section).Error; err != nil {
		t.Fatal(err)
	}

	svc := NewEnrollmentService(repository.NewEnrollmentRepository(db), db)
	v := models.StudentEnrollment{StudentID: student.ID, AcademicSessionID: session.ID, ClassID: class.ID, SectionID: section.ID}
	if _, err := svc.Create(v); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(v); err != ErrEnrollmentDuplicate {
		t.Fatalf("expected duplicate, got %v", err)
	}
}
