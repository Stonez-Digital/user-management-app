package repository

import (
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
 )

type StudentRepository interface {
    Create(models.Student) (models.Student, error)
    GetAll() ([]models.Student, error)
    GetByID(uuid.UUID) (models.Student, error)
    GetByUserID(uuid.UUID) (models.Student, error)
    GetByAdmissionNumber(string) (models.Student, error)
    Update(models.Student) error
    Delete(uuid.UUID) error
}

type studentRepo struct { db *gorm.DB }

func NewStudentRepository(db *gorm.DB) StudentRepository { return &studentRepo{db: db} }
func (r *studentRepo) Create(s models.Student) (models.Student, error) { if err := r.db.Create(&s).Error; err != nil { return s, err }; return s, nil }
func (r *studentRepo) GetAll() ([]models.Student, error) { var out []models.Student; err := r.db.Preload("User").Order("created_at DESC").Find(&out).Error; return out, err }
func (r *studentRepo) GetByID(id uuid.UUID) (models.Student, error) { var s models.Student; err := r.db.Preload("User").First(&s, "id = ?", id).Error; return s, err }
func (r *studentRepo) GetByUserID(id uuid.UUID) (models.Student, error) { var s models.Student; err := r.db.First(&s, "user_id = ?", id).Error; return s, err }
func (r *studentRepo) GetByAdmissionNumber(n string) (models.Student, error) { var s models.Student; err := r.db.First(&s, "admission_number = ?", n).Error; return s, err }
func (r *studentRepo) Update(s models.Student) error { return r.db.Save(&s).Error }
func (r *studentRepo) Delete(id uuid.UUID) error { return r.db.Delete(&models.Student{}, "id = ?", id).Error }