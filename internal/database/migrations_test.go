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
	if count != 1 {
		t.Fatalf("expected one applied migration, got %d", count)
	}

	if !db.Migrator().HasTable(&models.User{}) {
		t.Fatal("expected users table after migration")
	}
}
