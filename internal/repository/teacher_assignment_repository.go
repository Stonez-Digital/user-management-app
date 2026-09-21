package repository

import (
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

type TeacherAssignmentRepository interface {
	Create(uuid.UUID, models.TeacherAssignment) (models.TeacherAssignment, error)
	List(uuid.UUID) ([]models.TeacherAssignment, error)
	Get(uuid.UUID, uuid.UUID) (models.TeacherAssignment, error)
	Update(uuid.UUID, models.TeacherAssignment) error
	Delete(uuid.UUID, uuid.UUID) error
}

type teacherAssignmentRepo struct{ db *gorm.DB }

func NewTeacherAssignmentRepository(db *gorm.DB) TeacherAssignmentRepository { return &teacherAssignmentRepo{db: db} }

func (r *teacherAssignmentRepo) Create(schoolID uuid.UUID, v models.TeacherAssignment) (models.TeacherAssignment, error) {
	v.SchoolID = schoolID
	return v, r.db.Create(&v).Error
}

func (r *teacherAssignmentRepo) List(schoolID uuid.UUID) ([]models.TeacherAssignment, error) {
	var v []models.TeacherAssignment
	err := r.db.Where("school_id = ?", schoolID).
		Preload("Teacher", "school_id = ?", schoolID).
		Preload("Subject", "school_id = ?", schoolID).
		Preload("Class", "school_id = ?", schoolID).
		Preload("Section", "school_id = ?", schoolID).
		Order("created_at DESC").Find(&v).Error
	return v, err
}

func (r *teacherAssignmentRepo) Get(schoolID, id uuid.UUID) (models.TeacherAssignment, error) {
	var v models.TeacherAssignment
	err := r.db.Where("id = ? AND school_id = ?", id, schoolID).
		Preload("Teacher", "school_id = ?", schoolID).
		Preload("Subject", "school_id = ?", schoolID).
		Preload("Class", "school_id = ?", schoolID).
		Preload("Section", "school_id = ?", schoolID).
		First(&v).Error
	return v, err
}

func (r *teacherAssignmentRepo) Update(schoolID uuid.UUID, v models.TeacherAssignment) error {
	v.SchoolID = schoolID
	return r.db.Model(&models.TeacherAssignment{}).
		Where("id = ? AND school_id = ?", v.ID, schoolID).Updates(&v).Error
}

func (r *teacherAssignmentRepo) Delete(schoolID, id uuid.UUID) error {
	return r.db.Delete(&models.TeacherAssignment{}, "id = ? AND school_id = ?", id, schoolID).Error
}
