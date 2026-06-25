package database

import (
	"fmt"
	"log"

	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Connect() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("users.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect database:", err)
	}

	err = db.AutoMigrate(
		&models.User{},
		&models.RefreshToken{},
		&models.Session{},
		&models.PasswordResetToken{},
	)

	if err != nil {
		log.Fatal("migration failed:", err)
	}

	fmt.Println("Database connected + migrated")

	return db
}