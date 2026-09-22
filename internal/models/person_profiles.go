package models

import("time";"github.com/google/uuid";"gorm.io/gorm")

type TeacherProfile struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_teacher_staff,priority:1" json:"school_id"`
 UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
 StaffID string `gorm:"size:80;not null;uniqueIndex:uq_school_teacher_staff,priority:2" json:"staff_id"`
 Phone string `gorm:"size:30" json:"phone,omitempty"`
 Gender string `gorm:"size:30" json:"gender,omitempty"`
 EmploymentDate *time.Time `json:"employment_date,omitempty"`
 Department string `gorm:"size:100" json:"department,omitempty"`
 Designation string `gorm:"size:100" json:"designation,omitempty"`
 Subjects string `gorm:"type:text" json:"subjects,omitempty"`
 Classes string `gorm:"type:text" json:"classes,omitempty"`
 Status string `gorm:"size:30;not null;default:active" json:"status"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
func(t *TeacherProfile)BeforeCreate(tx *gorm.DB)error{t.ID=uuid.New();if t.Status==""{t.Status="active"};return nil}

type ParentProfile struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index" json:"school_id"`
 UserID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
 ParentIdentifier string `gorm:"size:100;index" json:"parent_identifier,omitempty"`
 Phone string `gorm:"size:30" json:"phone,omitempty"`
 Address string `gorm:"size:255" json:"address,omitempty"`
 Occupation string `gorm:"size:100" json:"occupation,omitempty"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
func(p *ParentProfile)BeforeCreate(tx *gorm.DB)error{p.ID=uuid.New();return nil}
