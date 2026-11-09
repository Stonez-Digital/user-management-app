package service

import (
    "testing"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func schoolTestService(t *testing.T) *SchoolService {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file:school_service_test_" + uuid.NewString() + "?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.School{}); err != nil { t.Fatal(err) }
    return NewSchoolService(repository.NewSchoolRepository(db), db)
}

func TestSchoolServiceReadsAndUpdatesOnlyRequestedSchool(t *testing.T) {
    s := schoolTestService(t)

    first := models.School{ID: uuid.New(), Name: "First School", Code: "FIRST", Status: models.SchoolStatusActive}
    second := models.School{ID: uuid.New(), Name: "Second School", Code: "SECOND", Status: models.SchoolStatusActive}
    db := s.DB()
    if err := db.Create(&first).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&second).Error; err != nil { t.Fatal(err) }

    school, err := s.GetSchool(first.ID)
    if err != nil { t.Fatal(err) }
    if school.Name != first.Name { t.Fatalf("expected first school, got %q", school.Name) }

    if err := s.UpdateSchool(first.ID, models.School{Name: "Updated First School"}); err != nil { t.Fatal(err) }

    updatedFirst, err := s.GetSchool(first.ID)
    if err != nil { t.Fatal(err) }
    updatedSecond, err := s.GetSchool(second.ID)
    if err != nil { t.Fatal(err) }

    if updatedFirst.Name != "Updated First School" { t.Fatalf("expected updated first school, got %q", updatedFirst.Name) }
    if updatedSecond.Name != second.Name { t.Fatalf("second school changed unexpectedly: %q", updatedSecond.Name) }
}

func TestSchoolServiceRejectsInvalidName(t *testing.T) {
    s := schoolTestService(t)
    school := models.School{ID: uuid.New(), Name: "School", Code: "SCHOOL", Status: models.SchoolStatusActive}
    if err := s.DB().Create(&school).Error; err != nil { t.Fatal(err) }

    if err := s.UpdateSchool(school.ID, models.School{Name: "   "}); err != ErrInvalidSchoolName {
        t.Fatalf("expected invalid school name, got %v", err)
    }
}

func TestSchoolServiceUpdatesOnlyRequestedSchoolLogo(t *testing.T) {
    s := schoolTestService(t)
    first := models.School{ID: uuid.New(), Name: "First School", Code: "FIRST", LogoURL: "old-first.png"}
    second := models.School{ID: uuid.New(), Name: "Second School", Code: "SECOND", LogoURL: "second.png"}
    if err := s.DB().Create(&first).Error; err != nil { t.Fatal(err) }
    if err := s.DB().Create(&second).Error; err != nil { t.Fatal(err) }

    if err := s.UpdateSchoolLogo(first.ID, " https://cdn.example.test/first.png "); err != nil { t.Fatal(err) }
    updatedFirst, err := s.GetSchool(first.ID); if err != nil { t.Fatal(err) }
    updatedSecond, err := s.GetSchool(second.ID); if err != nil { t.Fatal(err) }
    if updatedFirst.LogoURL != "https://cdn.example.test/first.png" { t.Fatalf("logo URL was not trimmed/persisted: %q", updatedFirst.LogoURL) }
    if updatedFirst.Name != first.Name { t.Fatalf("school profile changed unexpectedly: %q", updatedFirst.Name) }
    if updatedSecond.LogoURL != second.LogoURL { t.Fatalf("another school's logo changed: %q", updatedSecond.LogoURL) }
}

func TestSchoolServiceRejectsInvalidLogoURL(t *testing.T) {
    s := schoolTestService(t)
    school := models.School{ID: uuid.New(), Name: "School", Code: "SCHOOL"}
    if err := s.DB().Create(&school).Error; err != nil { t.Fatal(err) }
    if err := s.UpdateSchoolLogo(school.ID, " "); err != ErrInvalidSchoolLogo {
        t.Fatalf("expected invalid logo URL, got %v", err)
    }
}
