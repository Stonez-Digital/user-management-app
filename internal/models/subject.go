package models

import ("strings"; "time"; "github.com/google/uuid"; "gorm.io/gorm")

type Subject struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_subject_code,priority:1;uniqueIndex:uq_school_subject_name,priority:1" json:"school_id"`
 School School `gorm:"foreignKey:SchoolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
 Code string `gorm:"size:30;not null;uniqueIndex:uq_school_subject_code,priority:2" json:"code"`
 Name string `gorm:"size:100;not null;uniqueIndex:uq_school_subject_name,priority:2" json:"name"`
 Description string `gorm:"size:500" json:"description,omitempty"`
 Active bool `gorm:"not null;default:true;index" json:"active"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
func(s *Subject) BeforeCreate(tx *gorm.DB) error { s.ID=uuid.New(); s.Code=strings.ToUpper(strings.TrimSpace(s.Code)); s.Name=strings.TrimSpace(s.Name); return nil }