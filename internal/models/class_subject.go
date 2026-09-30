package models

import (
    "strings"
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

const (
    SubjectCategoryCore      = "core"
    SubjectCategoryOptional  = "optional"
    SubjectCategoryReligious = "religious"
    SubjectCategoryTrade     = "trade"
)

type ClassSubject struct {
    ID uuid.UUID \`gorm:"type:uuid;primaryKey" json:"id"\`
    SchoolID uuid.UUID \`gorm:"type:uuid;not null;index;uniqueIndex:uq_school_class_subject,priority:1" json:"school_id"\`
    School School \`gorm:"foreignKey:SchoolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"\`
    AcademicSessionID uuid.UUID \`gorm:"type:uuid;not null;index;uniqueIndex:uq_school_class_subject,priority:2" json:"academic_session_id"\`
    AcademicSession AcademicSession \`gorm:"foreignKey:AcademicSessionID" json:"-"\`
    ClassID uuid.UUID \`gorm:"type:uuid;not null;index;uniqueIndex:uq_school_class_subject,priority:3" json:"class_id"\`
    Class SchoolClass \`gorm:"foreignKey:ClassID" json:"class,omitempty"\`
    SubjectID uuid.UUID \`gorm:"type:uuid;not null;index;uniqueIndex:uq_school_class_subject,priority:4" json:"subject_id"\`
    Subject Subject \`gorm:"foreignKey:SubjectID" json:"subject,omitempty"\`
    CurriculumVersion string \`gorm:"size:100;not null;uniqueIndex:uq_school_class_subject,priority:5" json:"curriculum_version"\`
    Category string \`gorm:"size:20;not null;index" json:"category"\`
    Required bool \`gorm:"not null;default:true" json:"required"\`
    SelectionGroup string \`gorm:"size:50" json:"selection_group,omitempty"\`
    Active bool \`gorm:"not null;default:true;index" json:"active"\`
    CreatedAt time.Time \`json:"created_at"\`
    UpdatedAt time.Time \`json:"updated_at"\`
}

func (c *ClassSubject) BeforeCreate(tx *gorm.DB) error {
    c.ID = uuid.New()
    c.CurriculumVersion = strings.TrimSpace(c.CurriculumVersion)
    c.Category = strings.ToLower(strings.TrimSpace(c.Category))
    c.SelectionGroup = strings.TrimSpace(c.SelectionGroup)
    return nil
}
