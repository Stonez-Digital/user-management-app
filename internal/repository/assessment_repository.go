package repository

import (
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/gorm"
)
type AssessmentRepository interface {
 Create(uuid.UUID, models.Assessment)(models.Assessment,error)
 List(uuid.UUID)([]models.Assessment,error)
 Get(uuid.UUID,uuid.UUID)(models.Assessment,error)
 Update(uuid.UUID,models.Assessment)error
 Delete(uuid.UUID,uuid.UUID)error
}
type assessmentRepo struct{db *gorm.DB}
func NewAssessmentRepository(db *gorm.DB) AssessmentRepository{return &assessmentRepo{db}}
func(r *assessmentRepo)Create(schoolID uuid.UUID,v models.Assessment)(models.Assessment,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *assessmentRepo)List(schoolID uuid.UUID)([]models.Assessment,error){var v []models.Assessment;e:=r.db.Where("school_id = ?",schoolID).Preload("TeacherAssignment","school_id = ?",schoolID).Preload("TeacherAssignment.Subject","school_id = ?",schoolID).Preload("TeacherAssignment.Teacher","school_id = ?",schoolID).Order("date DESC, created_at DESC").Find(&v).Error;return v,e}
func(r *assessmentRepo)Get(schoolID,id uuid.UUID)(models.Assessment,error){var v models.Assessment;e:=r.db.Where("id = ? AND school_id = ?",id,schoolID).Preload("TeacherAssignment","school_id = ?",schoolID).Preload("TeacherAssignment.Subject","school_id = ?",schoolID).Preload("TeacherAssignment.Teacher","school_id = ?",schoolID).First(&v).Error;return v,e}
func(r *assessmentRepo)Update(schoolID uuid.UUID,v models.Assessment)error{v.SchoolID=schoolID;return r.db.Model(&models.Assessment{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(&v).Error}
func(r *assessmentRepo)Delete(schoolID,id uuid.UUID)error{return r.db.Delete(&models.Assessment{},"id = ? AND school_id = ?",id,schoolID).Error}
