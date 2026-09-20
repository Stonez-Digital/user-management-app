package repository

import (
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/gorm"
)
type AssessmentResultRepository interface {
 Create(uuid.UUID,models.AssessmentResult)(models.AssessmentResult,error)
 List(uuid.UUID)([]models.AssessmentResult,error)
 Get(uuid.UUID,uuid.UUID)(models.AssessmentResult,error)
 Update(uuid.UUID,models.AssessmentResult)error
 Delete(uuid.UUID,uuid.UUID)error
}
type assessmentResultRepo struct{db *gorm.DB}
func NewAssessmentResultRepository(db *gorm.DB) AssessmentResultRepository{return &assessmentResultRepo{db}}
func(r *assessmentResultRepo)Create(schoolID uuid.UUID,v models.AssessmentResult)(models.AssessmentResult,error){v.SchoolID=schoolID;return v,r.db.Create(&v).Error}
func(r *assessmentResultRepo)List(schoolID uuid.UUID)([]models.AssessmentResult,error){var v []models.AssessmentResult;e:=r.db.Where("school_id = ?",schoolID).Preload("Assessment","school_id = ?",schoolID).Preload("Assessment.TeacherAssignment","school_id = ?",schoolID).Preload("Assessment.TeacherAssignment.Subject","school_id = ?",schoolID).Preload("StudentEnrollment","school_id = ?",schoolID).Preload("StudentEnrollment.Student","school_id = ?",schoolID).Order("created_at DESC").Find(&v).Error;return v,e}
func(r *assessmentResultRepo)Get(schoolID,id uuid.UUID)(models.AssessmentResult,error){var v models.AssessmentResult;e:=r.db.Where("id = ? AND school_id = ?",id,schoolID).Preload("Assessment","school_id = ?",schoolID).Preload("Assessment.TeacherAssignment","school_id = ?",schoolID).Preload("Assessment.TeacherAssignment.Subject","school_id = ?",schoolID).Preload("StudentEnrollment","school_id = ?",schoolID).Preload("StudentEnrollment.Student","school_id = ?",schoolID).First(&v).Error;return v,e}
func(r *assessmentResultRepo)Update(schoolID uuid.UUID,v models.AssessmentResult)error{v.SchoolID=schoolID;return r.db.Model(&models.AssessmentResult{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(&v).Error}
func(r *assessmentResultRepo)Delete(schoolID,id uuid.UUID)error{return r.db.Delete(&models.AssessmentResult{},"id = ? AND school_id = ?",id,schoolID).Error}
