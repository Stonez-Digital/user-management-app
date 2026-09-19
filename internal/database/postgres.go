package database

import (
	"fmt"
	"os"

	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Connect() (*gorm.DB, error) {
	path := os.Getenv("DB_PATH")
	if path == "" {
		path = "users.db"
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	return db, nil
}

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&Migration{}); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	migrations := []MigrationStep{
		{
			Version: 1,
			Name:    "initial_school_management_schema",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(
					&models.User{},
					&models.RefreshToken{},
					&models.Session{},
					&models.PasswordResetToken{},
					&models.RoleChangeAudit{},
					&models.AuditLog{},
				)
			},
		},
	}

	for _, migration := range migrations {
		var applied Migration
		result := db.Where("version = ?", migration.Version).First(&applied)
		if result.Error == nil {
			continue
		}
		if result.Error != gorm.ErrRecordNotFound {
			return fmt.Errorf("check migration %d: %w", migration.Version, result.Error)
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := migration.Up(tx); err != nil {
				return err
			}
			return tx.Create(&Migration{
				Version: migration.Version,
				Name:    migration.Name,
			}).Error
		}); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}
	}

	return nil
}

type Migration struct {
	Version   int       `gorm:"primaryKey"`
	Name      string    `gorm:"not null;uniqueIndex"`
	AppliedAt interface{} `gorm:"-" json:"-"`
}

type MigrationStep struct {
	Version int
	Name    string
	Up      func(*gorm.DB) error
}
