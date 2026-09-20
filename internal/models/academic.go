package models

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

const (
    AcademicStatusPlanned = "planned"
    AcademicStatusActive = "active"
    AcademicStatusClosed = "closed"
    AcademicStatusArchived = "archived"
)
const (
    TermFirst = "first"
    TermSecond = "second"
    TermThird = "third"
)

type AcademicSession struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    SchoolID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_session_name,priority:1" json:"school_id"`
    School School `gorm:"foreignKey:SchoolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
    Name string `gorm:"size:100;not null;uniqueIndex:uq_school_session_name,priority:2" json:"name"`
    StartDate time.Time `gorm:"not null" json:"start_date"`
    EndDate time.Time `gorm:"not null" json:"end_date"`
    Status string `gorm:"size:20;not null;default:planned;index" json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Terms []Term `gorm:"foreignKey:AcademicSessionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"terms,omitempty"`
}
func (s *AcademicSession) BeforeCreate(tx *gorm.DB) error { s.ID=uuid.New(); if s.Status=="" {s.Status=AcademicStatusPlanned}; return nil }

type Term struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    SchoolID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_session_term,priority:1" json:"school_id"`
    School School `gorm:"foreignKey:SchoolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
    AcademicSessionID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_session_term,priority:2" json:"academic_session_id"`
    AcademicSession AcademicSession `gorm:"foreignKey:AcademicSessionID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
    Name string `gorm:"size:20;not null;uniqueIndex:uq_school_session_term,priority:3" json:"name"`
    StartDate time.Time `gorm:"not null" json:"start_date"`
    EndDate time.Time `gorm:"not null" json:"end_date"`
    Status string `gorm:"size:20;not null;default:planned;index" json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
func (t *Term) BeforeCreate(tx *gorm.DB) error { t.ID=uuid.New(); if t.Status=="" {t.Status=AcademicStatusPlanned}; return nil }