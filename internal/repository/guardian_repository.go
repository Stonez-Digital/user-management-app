package repository

import ("github.com/google/uuid"; "github.com/onoja217/users-management-app/internal/models"; "gorm.io/gorm")

type GuardianRepository interface {
 Create(uuid.UUID, models.GuardianRelationship) (models.GuardianRelationship,error)
 ListByGuardian(uuid.UUID,uuid.UUID) ([]models.GuardianRelationship,error)
 Find(uuid.UUID,uuid.UUID,uuid.UUID) (models.GuardianRelationship,error)
 Get(uuid.UUID,uuid.UUID) (models.GuardianRelationship,error)
 Update(models.GuardianRelationship) error
}
type guardianRepo struct{db *gorm.DB}
func NewGuardianRepository(db *gorm.DB) GuardianRepository{return &guardianRepo{db:db}}
func(r *guardianRepo)Create(schoolID uuid.UUID,v models.GuardianRelationship)(models.GuardianRelationship,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *guardianRepo)ListByGuardian(schoolID,gid uuid.UUID)([]models.GuardianRelationship,error){var v []models.GuardianRelationship;e:=r.db.Table("guardian_relationships").Joins("JOIN users ON users.id = guardian_relationships.guardian_user_id AND users.school_id = guardian_relationships.school_id AND users.active = ?",true).Where("guardian_relationships.school_id=? AND guardian_relationships.guardian_user_id=? AND guardian_relationships.active=?",schoolID,gid,true).Order("guardian_relationships.created_at").Find(&v).Error;return v,e}
func(r *guardianRepo)Find(schoolID,g,s uuid.UUID)(models.GuardianRelationship,error){var v models.GuardianRelationship;e:=r.db.Table("guardian_relationships").Joins("JOIN users ON users.id = guardian_relationships.guardian_user_id AND users.school_id = guardian_relationships.school_id AND users.active = ?",true).Where("guardian_relationships.school_id=? AND guardian_relationships.guardian_user_id=? AND guardian_relationships.student_id=? AND guardian_relationships.active=?",schoolID,g,s,true).First(&v).Error;return v,e}
func(r *guardianRepo)Get(schoolID,id uuid.UUID)(models.GuardianRelationship,error){var v models.GuardianRelationship;e:=r.db.Where("school_id=? AND id=?",schoolID,id).First(&v).Error;return v,e}
func(r *guardianRepo)Update(v models.GuardianRelationship)error{return r.db.Model(&models.GuardianRelationship{}).Where("school_id=? AND id=?",v.SchoolID,v.ID).Updates(map[string]interface{}{"relationship":v.Relationship,"primary":v.Primary,"active":v.Active}).Error}
