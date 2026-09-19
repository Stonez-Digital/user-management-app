package models

import (
    "time"
    "github.com/google/uuid"
)

type AuditLog struct {
    ID         uuid.UUID  `gorm:"type:uuid;primaryKey"`
    ActorID    *uuid.UUID `gorm:"type:uuid;index"`
    Action     string     `gorm:"not null;index"`
    Resource   string     `gorm:"not null;index"`
    ResourceID *uuid.UUID `gorm:"type:uuid;index"`
    IP         string
    Metadata   string
    CreatedAt  time.Time `gorm:"index"`
}
