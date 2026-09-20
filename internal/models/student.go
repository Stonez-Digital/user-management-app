package models

import (
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

const (
    EnrollmentActive = "active"
    EnrollmentInactive = "inactive"
    EnrollmentGraduated = "graduated"
    EnrollmentWithdrawn = "withdrawn"
)

type Student struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    SchoolID uuid.UUID `gorm:"type:uuid;not null;index" json:"school_id"`
    School School `gorm:"foreignKey:SchoolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
    UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
    User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
    AdmissionNumber string `gorm:"not null;uniqueIndex:uq_school_student_admission" json:"admission_number"`
    DateOfBirth *time.Time `json:"date_of_birth,omitempty"`
    Gender string `gorm:"size:30" json:"gender,omitempty"`
    GuardianName string `gorm:"size:100" json:"guardian_name,omitempty"`
    GuardianPhone string `gorm:"size:30" json:"guardian_phone,omitempty"`
    GuardianEmail string `gorm:"size:255" json:"guardian_email,omitempty"`
    EnrollmentStatus string `gorm:"not null;default:active;size:20" json:"enrollment_status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func (s *Student) BeforeCreate(tx *gorm.DB) error {
    s.ID = uuid.New()
    if s.EnrollmentStatus == "" { s.EnrollmentStatus = EnrollmentActive }
    return nil
}