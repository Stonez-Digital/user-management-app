package models

import("time";"github.com/google/uuid";"gorm.io/gorm")
type TeacherAssignment struct{
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 TeacherID uuid.UUID `gorm:"type:uuid;not null;index" json:"teacher_id"`
 Teacher User `gorm:"foreignKey:TeacherID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
 SubjectID uuid.UUID `gorm:"type:uuid;not null;index" json:"subject_id"`
 Subject Subject `gorm:"foreignKey:SubjectID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"subject,omitempty"`
 AcademicSessionID uuid.UUID `gorm:"type:uuid;not null;index" json:"academic_session_id"`
 TermID uuid.UUID `gorm:"type:uuid;not null;index" json:"term_id"`
 ClassID uuid.UUID `gorm:"type:uuid;not null;index" json:"class_id"`
 SectionID *uuid.UUID `gorm:"type:uuid;index" json:"section_id,omitempty"`
 Active bool `gorm:"not null;default:true;index" json:"active"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
func(t *TeacherAssignment)BeforeCreate(tx *gorm.DB)error{t.ID=uuid.New();return nil}