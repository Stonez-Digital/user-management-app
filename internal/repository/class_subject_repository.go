package repository

import (
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type ClassSubjectRepository interface {
    Create(uuid.UUID, models.ClassSubject) (models.ClassSubject, error)
    List(uuid.UUID, uuid.UUID, *uuid.UUID) ([]models.ClassSubject, error)
    Get(uuid.UUID, uuid.UUID) (models.ClassSubject, error)
    Update(uuid.UUID, models.ClassSubject) error
    Delete(uuid.UUID, uuid.UUID) error
}

type classSubjectRepo struct{ db *gorm.DB }

func NewClassSubjectRepository(db *gorm.DB) ClassSubjectRepository { return &classSubjectRepo{db} }

func (r *classSubjectRepo) Create(schoolID uuid.UUID, v models.ClassSubject) (models.ClassSubject, error) {
    v.SchoolID = schoolID
    return v, r.db.Create(&v).Error
}

func (r *classSubjectRepo) List(schoolID, sessionID uuid.UUID, classID *uuid.UUID) ([]models.ClassSubject, error) {
    var v []models.ClassSubject
    q := r.db.Where("school_id = ? AND academic_session_id = ?", schoolID, sessionID).
        Preload("Class").Preload("Subject").Order("class_id ASC, subject_id ASC")
    if classID != nil { q = q.Where("class_id = ?", *classID) }
    return v, q.Find(&v).Error
}

func (r *classSubjectRepo) Get(schoolID, id uuid.UUID) (models.ClassSubject, error) {
    var v models.ClassSubject
    e := r.db.Where("school_id = ?", schoolID).Preload("Class").Preload("Subject").First(&v, "id = ?", id).Error
    return v, e
}

func (r *classSubjectRepo) Update(schoolID uuid.UUID, v models.ClassSubject) error {
    v.SchoolID = schoolID
    return r.db.Model(&models.ClassSubject{}).Where("id = ? AND school_id = ?", v.ID, schoolID).Updates(&v).Error
}

func (r *classSubjectRepo) Delete(schoolID, id uuid.UUID) error {
    return r.db.Where("school_id = ?", schoolID).Delete(&models.ClassSubject{}, "id = ?", id).Error
}
