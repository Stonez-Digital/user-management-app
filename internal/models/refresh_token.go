package models

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;index"`
	Token     string    `gorm:"uniqueIndex"`
	ExpiresAt time.Time
	Revoked   bool      `gorm:"default:false"`
	CreatedAt time.Time
}