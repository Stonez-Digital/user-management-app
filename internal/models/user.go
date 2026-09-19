package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string `json:"name"`
	Email string `gorm:"uniqueIndex" json:"email"`
	PasswordHash string `json:"-"`
	Role string `gorm:"default:user" json:"role"`
	Active bool `gorm:"not null;default:true" json:"active"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	u.ID = uuid.New()
	return nil
}
