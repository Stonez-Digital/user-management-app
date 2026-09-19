package models

import ("time"; "github.com/google/uuid"; "gorm.io/gorm")

type Assessment struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 TeacherAssignmentID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_assessment_assignment_title" json:"teacher_assignment_id"`
 TeacherAssignment TeacherAssignment `gorm:"foreignKey:TeacherAssignmentID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"teacher_assignment,omitempty"`
 Title string `gorm:"size:150;not null;uniqueIndex:uq_assessment_assignment_title" json:"title"`
 Type string `gorm:"size:50;not null;index" json:"type"`
 MaxScore float64 `gorm:"not null" json:"max_score"`
 Weight float64 `gorm:"not null" json:"weight"`
 Date time.Time `gorm:"type:date;not null;index" json:"date"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
func (a *Assessment) BeforeCreate(tx *gorm.DB) error { a.ID=uuid.New(); return nil }
