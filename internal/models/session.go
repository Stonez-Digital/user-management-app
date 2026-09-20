package models

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID `gorm:"type:uuid;index"`
	RefreshTokenHash string `gorm:"uniqueIndex;size:64" json:"-"`

	ExpiresAt time.Time
	Revoked   bool `gorm:"default:false"`

	UserAgent string
	IP        string

	CreatedAt time.Time
}
