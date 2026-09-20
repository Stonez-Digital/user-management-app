package models

import ("time"; "github.com/google/uuid"; "gorm.io/gorm")

const ( EnrollmentStatusActive="active"; EnrollmentStatusCompleted="completed"; EnrollmentStatusWithdrawn="withdrawn" )

type StudentEnrollment struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_student_session" json:"school_id"`
 School School `gorm:"foreignKey:SchoolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
 StudentID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_student_session" json:"student_id"`
 AcademicSessionID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_student_session" json:"academic_session_id"`
 ClassID uuid.UUID `gorm:"type:uuid;not null;index" json:"class_id"`
 SectionID uuid.UUID `gorm:"type:uuid;not null;index" json:"section_id"`
 Status string `gorm:"size:20;not null;default:active;index" json:"status"`
 EnrolledAt time.Time `gorm:"not null" json:"enrolled_at"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
 Student Student `gorm:"foreignKey:StudentID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"student,omitempty"`
 AcademicSession AcademicSession `gorm:"foreignKey:AcademicSessionID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"academic_session,omitempty"`
 SchoolClass SchoolClass `gorm:"foreignKey:ClassID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"class,omitempty"`
 Section Section `gorm:"foreignKey:SectionID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"section,omitempty"`
}
func(e *StudentEnrollment)BeforeCreate(tx *gorm.DB)error{e.ID=uuid.New();if e.Status==""{e.Status=EnrollmentStatusActive};if e.EnrolledAt.IsZero(){e.EnrolledAt=time.Now().UTC()};return nil}