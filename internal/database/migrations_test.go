package database

import (
	"testing"

	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrateIsVersionedAndIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := db.Model(&Migration{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Fatalf("expected five applied migrations, got %d", count)
	}

	if !db.Migrator().HasTable(&models.User{}) {
		t.Fatal("expected users table after migration")
	}
	if !db.Migrator().HasTable(&models.Student{}) {
		t.Fatal("expected students table after migration")
	}
	if !db.Migrator().HasTable(&models.StudentEnrollment{}) {
		t.Fatal("expected student enrollments table after migration")
	}
}
