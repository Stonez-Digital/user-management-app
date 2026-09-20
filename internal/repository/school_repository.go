package repository

import (
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type SchoolRepository interface {
    GetByID(uuid.UUID) (models.School, error)
    Update(uuid.UUID, models.School) error
}

type schoolRepo struct{ db *gorm.DB }

func NewSchoolRepository(db *gorm.DB) SchoolRepository { return &schoolRepo{db: db} }

func (r *schoolRepo) GetByID(id uuid.UUID) (models.School, error) {
    var school models.School
    err := r.db.First(&school, "id = ?", id).Error
    return school, err
}

func (r *schoolRepo) Update(id uuid.UUID, school models.School) error {
    result := r.db.Model(&models.School{}).Where("id = ?", id).Updates(map[string]interface{}{"name": school.Name})
    if result.Error != nil { return result.Error }
    if result.RowsAffected == 0 { return gorm.ErrRecordNotFound }
    return nil
}
