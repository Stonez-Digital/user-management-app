package repository

import ("github.com/google/uuid"; "github.com/onoja217/users-management-app/internal/models"; "gorm.io/gorm")

type GuardianRepository interface {
 Create(uuid.UUID, models.GuardianRelationship) (models.GuardianRelationship,error)
 ListByGuardian(uuid.UUID,uuid.UUID) ([]models.GuardianRelationship,error)
 Find(uuid.UUID,uuid.UUID,uuid.UUID) (models.GuardianRelationship,error)
}
type guardianRepo struct{db *gorm.DB}
func NewGuardianRepository(db *gorm.DB) GuardianRepository{return &guardianRepo{db:db}}
func(r *guardianRepo)Create(schoolID uuid.UUID,v models.GuardianRelationship)(models.GuardianRelationship,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *guardianRepo)ListByGuardian(schoolID,gid uuid.UUID)([]models.GuardianRelationship,error){var v []models.GuardianRelationship;e:=r.db.Where("school_id=? AND guardian_user_id=? AND active=?",schoolID,gid,true).Order("created_at").Find(&v).Error;return v,e}
func(r *guardianRepo)Find(schoolID,g,s uuid.UUID)(models.GuardianRelationship,error){var v models.GuardianRelationship;e:=r.db.Where("school_id=? AND guardian_user_id=? AND student_id=? AND active=?",schoolID,g,s,true).First(&v).Error;return v,e}
