package models

import ("time"; "github.com/google/uuid"; "gorm.io/gorm")

type SchoolClass struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 Name string `gorm:"size:100;not null;uniqueIndex" json:"name"`
 Level int `gorm:"not null;index" json:"level"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
 Sections []Section `gorm:"foreignKey:ClassID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"sections,omitempty"`
}
func(c *SchoolClass) BeforeCreate(tx *gorm.DB) error { c.ID=uuid.New(); return nil }

type Section struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 ClassID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_class_section" json:"class_id"`
 Name string `gorm:"size:50;not null;uniqueIndex:uq_class_section" json:"name"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
func(s *Section) BeforeCreate(tx *gorm.DB) error { s.ID=uuid.New(); return nil }
