package service

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
)

func TestEnrollmentListAndHistorySerializeStudentName(t *testing.T) {
	db := enrollmentTestDB(t)
	schoolID := uuid.New()
	user := models.User{SchoolID: &schoolID, Name: "Jane Doe", Email: "jane-"+schoolID.String()+"@example.com", Role: "student", Active: true}
	if err := db.Create(&user).Error; err != nil { t.Fatal(err) }
	student := models.Student{SchoolID: schoolID, UserID: user.ID, AdmissionNumber: "KG001"}
	if err := db.Create(&student).Error; err != nil { t.Fatal(err) }
	session := models.AcademicSession{SchoolID: schoolID, Name: "2026/2027"}
	if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
	class := models.SchoolClass{SchoolID: schoolID, Name: "JSS1", Level: 1}
	if err := db.Create(&class).Error; err != nil { t.Fatal(err) }
	section := models.Section{SchoolID: schoolID, ClassID: class.ID, Name: "A"}
	if err := db.Create(&section).Error; err != nil { t.Fatal(err) }

	svc := NewEnrollmentService(repository.NewEnrollmentRepository(db), db)
	enrollment, err := svc.Create(schoolID, models.StudentEnrollment{StudentID: student.ID, AcademicSessionID: session.ID, ClassID: class.ID, SectionID: section.ID})
	if err != nil { t.Fatal(err) }
	if enrollment.Student.User.Name != "Jane Doe" { t.Fatalf("expected create response to include student name, got %#v", enrollment.Student.User) }

	for _, read := range []struct{name string; fn func() ([]models.StudentEnrollment, error)}{
		{name: "list", fn: func() ([]models.StudentEnrollment, error) { return svc.List(schoolID) }},
		{name: "history", fn: func() ([]models.StudentEnrollment, error) { return svc.History(schoolID, student.ID) }},
	} {
		t.Run(read.name, func(t *testing.T) {
			rows, err := read.fn()
			if err != nil { t.Fatal(err) }
			raw, err := json.Marshal(rows)
			if err != nil { t.Fatal(err) }
			var payload []struct { Student struct { User struct { Name string `json:"name"` } `json:"user"` } `json:"student"` }
			if err := json.Unmarshal(raw, &payload); err != nil { t.Fatal(err) }
			if len(payload) != 1 || payload[0].Student.User.Name != "Jane Doe" {
				t.Fatalf("expected student name in enrollment %s JSON, got %s", read.name, raw)
			}
		})
	}
}
