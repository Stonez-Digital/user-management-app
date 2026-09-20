package models

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

type AssessmentResult struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    AssessmentID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_result_assessment_enrollment" json:"assessment_id"`
    Assessment Assessment `gorm:"foreignKey:AssessmentID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"assessment,omitempty"`
    StudentEnrollmentID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_result_assessment_enrollment" json:"student_enrollment_id"`
    StudentEnrollment StudentEnrollment `gorm:"foreignKey:StudentEnrollmentID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"student_enrollment,omitempty"`
    Score float64 `gorm:"not null" json:"score"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func (r *AssessmentResult) BeforeCreate(tx *gorm.DB) error {
    r.ID = uuid.New()
    return nil
}
