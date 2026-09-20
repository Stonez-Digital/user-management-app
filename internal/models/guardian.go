package models

import("time";"github.com/google/uuid";"gorm.io/gorm")
type GuardianRelationship struct{ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`;GuardianUserID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:ux_guardian_student" json:"guardian_user_id"`;StudentID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:ux_guardian_student" json:"student_id"`;Relationship string `gorm:"size:40;not null" json:"relationship"`;Primary bool `gorm:"not null;default:false" json:"primary"`;Active bool `gorm:"not null;default:true;index" json:"active"`;CreatedAt time.Time `json:"created_at"`;UpdatedAt time.Time `json:"updated_at"`}
func(g *GuardianRelationship)BeforeCreate(tx *gorm.DB)error{g.ID=uuid.New();return nil}
