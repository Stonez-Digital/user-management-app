package repository

import (
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type StudentRepository interface {
    Create(uuid.UUID, models.Student) (models.Student, error)
    GetAll(uuid.UUID) ([]models.Student, error)
    GetByID(uuid.UUID, uuid.UUID) (models.Student, error)
    GetByUserID(uuid.UUID, uuid.UUID) (models.Student, error)
    GetByAdmissionNumber(uuid.UUID, string) (models.Student, error)
    Update(uuid.UUID, models.Student) error
    Delete(uuid.UUID, uuid.UUID) error
}

type studentRepo struct { db *gorm.DB }

func NewStudentRepository(db *gorm.DB) StudentRepository { return &studentRepo{db: db} }
func (r *studentRepo) Create(schoolID uuid.UUID, s models.Student) (models.Student, error) { s.SchoolID = schoolID; if err := r.db.Create(&s).Error; err != nil { return s, err }; return s, nil }
func (r *studentRepo) GetAll(schoolID uuid.UUID) ([]models.Student, error) { var out []models.Student; err := r.db.Where("school_id = ?", schoolID).Preload("User").Order("created_at DESC").Find(&out).Error; return out, err }
func (r *studentRepo) GetByID(schoolID, id uuid.UUID) (models.Student, error) { var s models.Student; err := r.db.Where("school_id = ?", schoolID).Preload("User").First(&s, "id = ?", id).Error; return s, err }
func (r *studentRepo) GetByUserID(schoolID, id uuid.UUID) (models.Student, error) { var s models.Student; err := r.db.Where("school_id = ?", schoolID).First(&s, "user_id = ?", id).Error; return s, err }
func (r *studentRepo) GetByAdmissionNumber(schoolID uuid.UUID, n string) (models.Student, error) { var s models.Student; err := r.db.Where("school_id = ? AND admission_number = ?", schoolID, n).First(&s).Error; return s, err }
func (r *studentRepo) Update(schoolID uuid.UUID, s models.Student) error { s.SchoolID = schoolID; return r.db.Model(&models.Student{}).Where("id = ? AND school_id = ?", s.ID, schoolID).Updates(&s).Error }
func (r *studentRepo) Delete(schoolID, id uuid.UUID) error { return r.db.Where("school_id = ?", schoolID).Delete(&models.Student{}, "id = ?", id).Error }
