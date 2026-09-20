package models

import ("time"; "github.com/google/uuid"; "gorm.io/gorm")

type SchoolClass struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_class_name,priority:1" json:"school_id"`
 School School `gorm:"foreignKey:SchoolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
 Name string `gorm:"size:100;not null;uniqueIndex:uq_school_class_name,priority:2" json:"name"`
 Level int `gorm:"not null;index" json:"level"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
 Sections []Section `gorm:"foreignKey:ClassID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"sections,omitempty"`
}
func(c *SchoolClass) BeforeCreate(tx *gorm.DB) error { c.ID=uuid.New(); return nil }

type Section struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_class_section,priority:1" json:"school_id"`
 School School `gorm:"foreignKey:SchoolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
 ClassID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_class_section,priority:2" json:"class_id"`
 Name string `gorm:"size:50;not null;uniqueIndex:uq_school_class_section,priority:3" json:"name"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
func(s *Section) BeforeCreate(tx *gorm.DB) error { s.ID=uuid.New(); return nil }