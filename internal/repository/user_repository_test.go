package repository

import (
    "errors"
    "testing"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func userRepositoryTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil {
        t.Fatal(err)
    }
    if err := db.AutoMigrate(&models.School{}, &models.User{}); err != nil {
        t.Fatal(err)
    }
    return db
}

func TestUserRepositoryEnforcesSchoolIsolation(t *testing.T) {
    db := userRepositoryTestDB(t)
    repo := NewUserRepository(db)

    schoolA := models.School{Name: "School A", Code: "A", Status: models.SchoolStatusActive}
    schoolB := models.School{Name: "School B", Code: "B", Status: models.SchoolStatusActive}
    if err := db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&schoolB).Error; err != nil { t.Fatal(err) }

    userA, err := repo.Create(schoolA.ID, models.User{Name: "A User", Email: "a@example.com", Role: "student", Active: true})
    if err != nil { t.Fatal(err) }
    userB, err := repo.Create(schoolB.ID, models.User{Name: "B User", Email: "b@example.com", Role: "student", Active: true})
    if err != nil { t.Fatal(err) }

    usersA, err := repo.GetAll(schoolA.ID)
    if err != nil { t.Fatal(err) }
    if len(usersA) != 1 || usersA[0].ID != userA.ID {
        t.Fatalf("expected only School A user, got %#v", usersA)
    }

    if _, err := repo.GetByID(schoolA.ID, userB.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
        t.Fatalf("expected cross-school lookup to be rejected, got %v", err)
    }

    userA.Name = "A User Updated"
    userA.SchoolID = &schoolB.ID
    if err := repo.Update(schoolA.ID, userA); err != nil { t.Fatal(err) }
    var unchanged models.User
    if err := db.First(&unchanged, "id = ?", userB.ID).Error; err != nil { t.Fatal(err) }
    if unchanged.Name != "B User" {
        t.Fatal("cross-school update affected School B user")
    }

    if err := repo.Delete(schoolA.ID, userB.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
        t.Fatalf("expected cross-school delete to be rejected, got %v", err)
    }
    if err := db.First(&unchanged, "id = ?", userB.ID).Error; err != nil {
        t.Fatalf("expected School B user to remain: %v", err)
    }

    // Verify a generated ID remains stable and tenant ownership is server-assigned.
    if userA.ID == uuid.Nil || userB.ID == uuid.Nil {
        t.Fatal("expected users to have generated IDs")
    }
    var stored models.User
    if err := db.First(&stored, "id = ?", userA.ID).Error; err != nil { t.Fatal(err) }
    if stored.SchoolID == nil || *stored.SchoolID != schoolA.ID {
        t.Fatal("expected repository create/update to force School A ownership")
    }
    if stored.Name != "A User Updated" {
        t.Fatal("expected School A user update to persist")
    }
}
