package repository

import ("github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"gorm.io/gorm")
type SubjectRepository interface{Create(models.Subject)(models.Subject,error);List()([]models.Subject,error);Get(uuid.UUID)(models.Subject,error);Update(models.Subject)error;Delete(uuid.UUID)error}
type subjectRepo struct{db *gorm.DB}
func NewSubjectRepository(db *gorm.DB)SubjectRepository{return &subjectRepo{db}}
func(r *subjectRepo)Create(v models.Subject)(models.Subject,error){return v,r.db.Create(&v).Error}
func(r *subjectRepo)List()([]models.Subject,error){var v []models.Subject;e:=r.db.Order("name ASC").Find(&v).Error;return v,e}
func(r *subjectRepo)Get(id uuid.UUID)(models.Subject,error){var v models.Subject;e:=r.db.First(&v,"id = ?",id).Error;return v,e}
func(r *subjectRepo)Update(v models.Subject)error{return r.db.Save(&v).Error}
func(r *subjectRepo)Delete(id uuid.UUID)error{return r.db.Delete(&models.Subject{},"id = ?",id).Error}
