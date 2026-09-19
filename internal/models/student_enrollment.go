package models

import("time";"github.com/google/uuid";"gorm.io/gorm")
const(EnrollmentStatusActive="active";EnrollmentStatusCompleted="completed";EnrollmentStatusWithdrawn="withdrawn")
type StudentEnrollment struct{
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 StudentID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_student_session" json:"student_id"`
 AcademicSessionID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_student_session" json:"academic_session_id"`
 ClassID uuid.UUID `gorm:"type:uuid;not null;index" json:"class_id"`
 SectionID uuid.UUID `gorm:"type:uuid;not null;index" json:"section_id"`
 Status string `gorm:"size:20;not null;default:active;index" json:"status"`
 EnrolledAt time.Time `gorm:"not null" json:"enrolled_at"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
func(e *StudentEnrollment)BeforeCreate(tx *gorm.DB)error{e.ID=uuid.New();if e.Status==""{e.Status=EnrollmentStatusActive};if e.EnrolledAt.IsZero(){e.EnrolledAt=time.Now()};return nil}