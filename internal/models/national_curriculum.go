package models

import (
 "strings"
 "time"
 "github.com/google/uuid"
 "gorm.io/gorm"
)

const (
 NationalCurriculumStatusDraft="draft"
 NationalCurriculumStatusActive="active"
 NationalCurriculumStatusRetired="retired"
 NationalCurriculumCategoryCore="core"
 NationalCurriculumCategoryOptional="optional"
 NationalCurriculumCategoryReligious="religious"
 NationalCurriculumCategoryTrade="trade"
)

type NationalCurriculum struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 Code string `gorm:"size:50;not null;uniqueIndex" json:"code"`
 Name string `gorm:"size:200;not null" json:"name"`
 Authority string `gorm:"size:120;not null" json:"authority"`
 Version string `gorm:"size:80;not null" json:"version"`
 Description string `gorm:"type:text" json:"description,omitempty"`
 EffectiveFrom *time.Time `json:"effective_from,omitempty"`
 EffectiveTo *time.Time `json:"effective_to,omitempty"`
 Status string `gorm:"size:20;not null;default:draft;index" json:"status"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
 Levels []NationalCurriculumLevel `gorm:"foreignKey:CurriculumID" json:"levels,omitempty"`
}
func (n *NationalCurriculum) BeforeCreate(tx *gorm.DB) error { n.ID=uuid.New(); n.Code=strings.ToUpper(strings.TrimSpace(n.Code)); n.Name=strings.TrimSpace(n.Name); n.Authority=strings.TrimSpace(n.Authority); n.Version=strings.TrimSpace(n.Version); if n.Status=="" {n.Status=NationalCurriculumStatusDraft}; return nil }

type NationalCurriculumLevel struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 CurriculumID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_national_curriculum_level,priority:1" json:"curriculum_id"`
 Code string `gorm:"size:30;not null;uniqueIndex:uq_national_curriculum_level,priority:2" json:"code"`
 Name string `gorm:"size:100;not null" json:"name"`
 DisplayOrder int `gorm:"not null" json:"display_order"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
 Classes []NationalCurriculumClass `gorm:"foreignKey:LevelID" json:"classes,omitempty"`
}
func (n *NationalCurriculumLevel) BeforeCreate(tx *gorm.DB) error { n.ID=uuid.New(); n.Code=strings.ToUpper(strings.TrimSpace(n.Code)); n.Name=strings.TrimSpace(n.Name); return nil }

type NationalCurriculumClass struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 LevelID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_national_curriculum_class,priority:1" json:"level_id"`
 Code string `gorm:"size:30;not null;uniqueIndex:uq_national_curriculum_class,priority:2" json:"code"`
 Name string `gorm:"size:100;not null" json:"name"`
 DisplayOrder int `gorm:"not null" json:"display_order"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
 Subjects []NationalCurriculumSubject `gorm:"foreignKey:ClassID" json:"subjects,omitempty"`
}
func (n *NationalCurriculumClass) BeforeCreate(tx *gorm.DB) error { n.ID=uuid.New(); n.Code=strings.ToUpper(strings.TrimSpace(n.Code)); n.Name=strings.TrimSpace(n.Name); return nil }

type NationalCurriculumSubject struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 ClassID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_national_curriculum_subject,priority:1" json:"class_id"`
 Code string `gorm:"size:40;not null;uniqueIndex:uq_national_curriculum_subject,priority:2" json:"code"`
 Name string `gorm:"size:150;not null;uniqueIndex:uq_national_curriculum_subject,priority:3" json:"name"`
 Category string `gorm:"size:20;not null;index" json:"category"`
 Required bool `gorm:"not null;default:true" json:"required"`
 SelectionGroup *string `gorm:"size:60" json:"selection_group,omitempty"`
 EffectiveFrom *time.Time `json:"effective_from,omitempty"`
 EffectiveTo *time.Time `json:"effective_to,omitempty"`
 Active bool `gorm:"not null;default:true;index" json:"active"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
func (n *NationalCurriculumSubject) BeforeCreate(tx *gorm.DB) error { n.ID=uuid.New(); n.Code=strings.ToUpper(strings.TrimSpace(n.Code)); n.Name=strings.TrimSpace(n.Name); n.Category=strings.ToLower(strings.TrimSpace(n.Category)); return nil }
