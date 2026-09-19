package repository

import (
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type GuardianRepository interface {
    Create(models.GuardianRelationship)(models.GuardianRelationship,error)
    ListByGuardian(uuid.UUID)([]models.GuardianRelationship,error)
    Find(uuid.UUID,uuid.UUID)(models.GuardianRelationship,error)
}
type guardianRepo struct{db *gorm.DB}
func NewGuardianRepository(db *gorm.DB) GuardianRepository{return &guardianRepo{db}}
func(r *guardianRepo)Create(v models.GuardianRelationship)(models.GuardianRelationship,error){return v,r.db.Create(&v).Error}
func(r *guardianRepo)ListByGuardian(id uuid.UUID)([]models.GuardianRelationship,error){var v []models.GuardianRelationship;e:=r.db.Where("guardian_user_id=? AND active=true",id).Order("created_at").Find(&v).Error;return v,e}
func(r *guardianRepo)Find(g,s uuid.UUID)(models.GuardianRelationship,error){var v models.GuardianRelationship;e:=r.db.Where("guardian_user_id=? AND student_id=? AND active=true",g,s).First(&v).Error;return v,e}