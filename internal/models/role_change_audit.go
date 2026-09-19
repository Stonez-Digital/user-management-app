package models

import (
    "time"
    "github.com/google/uuid"
)

type RoleChangeAudit struct {
    ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
    ActorID   uuid.UUID `gorm:"type:uuid;index;not null"`
    TargetID  uuid.UUID `gorm:"type:uuid;index;not null"`
    FromRole  string    `gorm:"not null"`
    ToRole    string    `gorm:"not null"`
    IP        string
    CreatedAt time.Time
}
