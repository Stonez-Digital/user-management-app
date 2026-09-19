package repository

import (
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

type EnrollmentRepository interface {
	Create(models.StudentEnrollment) (models.StudentEnrollment, error)
	List() ([]models.StudentEnrollment, error)
	Get(uuid.UUID) (models.StudentEnrollment, error)
	Update(models.StudentEnrollment) error
	Delete(uuid.UUID) error
}

type enrollmentRepo struct{ db *gorm.DB }

func NewEnrollmentRepository(db *gorm.DB) EnrollmentRepository { return &enrollmentRepo{db: db} }

func (r *enrollmentRepo) Create(v models.StudentEnrollment) (models.StudentEnrollment, error) {
	return v, r.db.Create(&v).Error
}
func (r *enrollmentRepo) List() ([]models.StudentEnrollment, error) {
	var out []models.StudentEnrollment
	err := r.db.Preload("Student.User").Preload("AcademicSession").Preload("SchoolClass").Preload("Section").Order("enrolled_at DESC").Find(&out).Error
	return out, err
}
func (r *enrollmentRepo) Get(id uuid.UUID) (models.StudentEnrollment, error) {
	var v models.StudentEnrollment
	err := r.db.Preload("Student.User").Preload("AcademicSession").Preload("SchoolClass").Preload("Section").First(&v, "id = ?", id).Error
	return v, err
}
func (r *enrollmentRepo) Update(v models.StudentEnrollment) error { return r.db.Save(&v).Error }
func (r *enrollmentRepo) Delete(id uuid.UUID) error { return r.db.Delete(&models.StudentEnrollment{}, "id = ?", id).Error }
