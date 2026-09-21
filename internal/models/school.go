package models

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

const (
    SchoolStatusPending = "pending"
    SchoolStatusActive = "active"
    SchoolStatusSuspended = "suspended"
)

type School struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    Name string `gorm:"size:160;not null" json:"name"`
    Code string `gorm:"size:50;not null;uniqueIndex" json:"code"`
    Status string `gorm:"size:20;not null;default:active;index" json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Users []User `gorm:"foreignKey:SchoolID" json:"-"`
}

func (s *School) BeforeCreate(tx *gorm.DB) error {
    if s.ID == uuid.Nil { s.ID = uuid.New() }
    return nil
}
