package repository

import (
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

type EnrollmentRepository interface {
	Create(uuid.UUID, models.StudentEnrollment) (models.StudentEnrollment, error)
	List(uuid.UUID) ([]models.StudentEnrollment, error)
	Get(uuid.UUID, uuid.UUID) (models.StudentEnrollment, error)
	Update(uuid.UUID, models.StudentEnrollment) error
	Delete(uuid.UUID, uuid.UUID) error
}

type enrollmentRepo struct{ db *gorm.DB }
func NewEnrollmentRepository(db *gorm.DB) EnrollmentRepository { return &enrollmentRepo{db: db} }
func (r *enrollmentRepo) Create(schoolID uuid.UUID, v models.StudentEnrollment) (models.StudentEnrollment, error) { v.SchoolID=schoolID; return v, r.db.Create(&v).Error }
func (r *enrollmentRepo) List(schoolID uuid.UUID) ([]models.StudentEnrollment, error) { var out []models.StudentEnrollment; err := r.db.Where("school_id = ?",schoolID).Preload("Student.User","school_id = ?",schoolID).Preload("AcademicSession","school_id = ?",schoolID).Preload("SchoolClass","school_id = ?",schoolID).Preload("Section","school_id = ?",schoolID).Order("enrolled_at DESC").Find(&out).Error; return out, err }
func (r *enrollmentRepo) Get(schoolID,id uuid.UUID) (models.StudentEnrollment, error) { var v models.StudentEnrollment; err := r.db.Where("school_id = ?",schoolID).Preload("Student.User","school_id = ?",schoolID).Preload("AcademicSession","school_id = ?",schoolID).Preload("SchoolClass","school_id = ?",schoolID).Preload("Section","school_id = ?",schoolID).First(&v,"id = ?",id).Error; return v, err }
func (r *enrollmentRepo) Update(schoolID uuid.UUID,v models.StudentEnrollment) error { v.SchoolID=schoolID; return r.db.Model(&models.StudentEnrollment{}).Where("id = ? AND school_id = ?",v.ID,schoolID).Updates(&v).Error }
func (r *enrollmentRepo) Delete(schoolID,id uuid.UUID) error { return r.db.Where("school_id = ?",schoolID).Delete(&models.StudentEnrollment{},"id = ?",id).Error }
