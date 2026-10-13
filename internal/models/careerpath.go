package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CareerPathAttempt stores a student's private, versioned exploratory assessment.
// Response and result payloads are JSON strings to keep the first schema additive;
// the service validates their shape and never accepts ownership fields from clients.
type CareerPathAttempt struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	SchoolID    uuid.UUID  `gorm:"type:uuid;not null;index:idx_career_attempt_owner,priority:1" json:"-"`
	StudentID   uuid.UUID  `gorm:"type:uuid;not null;index:idx_career_attempt_owner,priority:2" json:"-"`
	UserID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"-"`
	Version     string     `gorm:"size:32;not null" json:"version"`
	Status      string     `gorm:"size:20;not null;index" json:"status"`
	Responses   string     `gorm:"type:text;not null;default:'{}'" json:"-"`
	Results     string     `gorm:"type:text;not null;default:'[]'" json:"-"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (a *CareerPathAttempt) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.Version == "" {
		a.Version = "careerpath-v1"
	}
	if a.Status == "" {
		a.Status = "in_progress"
	}
	if a.Responses == "" {
		a.Responses = "{}"
	}
	if a.Results == "" {
		a.Results = "[]"
	}
	return nil
}
