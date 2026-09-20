package repository

import ("github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"gorm.io/gorm")
type SubjectRepository interface{Create(uuid.UUID,models.Subject)(models.Subject,error);List(uuid.UUID)([]models.Subject,error);Get(uuid.UUID,uuid.UUID)(models.Subject,error);Update(uuid.UUID,models.Subject)error;Delete(uuid.UUID,uuid.UUID)error}
type subjectRepo struct{db *gorm.DB}
func NewSubjectRepository(db *gorm.DB)SubjectRepository{return &subjectRepo{db}}
func(r *subjectRepo)Create(schoolID uuid.UUID,v models.Subject)(models.Subject,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *subjectRepo)List(schoolID uuid.UUID)([]models.Subject,error){var v []models.Subject;e:=r.db.Where("school_id = ?",schoolID).Order("name ASC").Find(&v).Error;return v,e}
func(r *subjectRepo)Get(schoolID,id uuid.UUID)(models.Subject,error){var v models.Subject;e:=r.db.Where("school_id = ?",schoolID).First(&v,"id = ?",id).Error;return v,e}
func(r *subjectRepo)Update(schoolID uuid.UUID,v models.Subject)error{v.SchoolID=schoolID;return r.db.Model(&models.Subject{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(&v).Error}
func(r *subjectRepo)Delete(schoolID,id uuid.UUID)error{return r.db.Where("school_id = ?",schoolID).Delete(&models.Subject{},"id = ?",id).Error}
