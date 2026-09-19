package models

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

const (
    TimetableMonday = 1
    TimetableTuesday = 2
    TimetableWednesday = 3
    TimetableThursday = 4
    TimetableFriday = 5
    TimetableSaturday = 6
)

type TimetableEntry struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    AcademicSessionID uuid.UUID `gorm:"type:uuid;not null;index" json:"academic_session_id"`
    TermID uuid.UUID `gorm:"type:uuid;not null;index" json:"term_id"`
    TeacherAssignmentID uuid.UUID `gorm:"type:uuid;not null;index" json:"teacher_assignment_id"`
    ClassID uuid.UUID `gorm:"type:uuid;not null;index" json:"class_id"`
    SectionID *uuid.UUID `gorm:"type:uuid;index" json:"section_id,omitempty"`
    DayOfWeek int `gorm:"not null;index" json:"day_of_week"`
    StartTime string `gorm:"size:5;not null" json:"start_time"`
    EndTime string `gorm:"size:5;not null" json:"end_time"`
    Room string `gorm:"size:80" json:"room,omitempty"`
    Active bool `gorm:"not null;default:true;index" json:"active"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
func (t *TimetableEntry) BeforeCreate(tx *gorm.DB) error { t.ID=uuid.New(); return nil }