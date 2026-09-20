package models

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	FamilyID  uuid.UUID `gorm:"type:uuid;index"
	UserID    uuid.UUID `gorm:"type:uuid;index"`
	TokenHash string    `gorm:"size:64"`
	ExpiresAt time.Time
	Revoked   bool      `gorm:"default:false"`
	CreatedAt time.Time
}