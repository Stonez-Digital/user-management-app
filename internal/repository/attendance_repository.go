package repository

import (
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

type AttendanceRepository interface {
	Create(models.AttendanceRecord) (models.AttendanceRecord, error)
	List() ([]models.AttendanceRecord, error)
	Get(uuid.UUID) (models.AttendanceRecord, error)
	Update(models.AttendanceRecord) error
	Delete(uuid.UUID) error
}

type attendanceRepo struct{ db *gorm.DB }

func NewAttendanceRepository(db *gorm.DB) AttendanceRepository { return &attendanceRepo{db: db} }

func (r *attendanceRepo) Create(v models.AttendanceRecord) (models.AttendanceRecord, error) {
	return v, r.db.Create(&v).Error
}
func (r *attendanceRepo) List() ([]models.AttendanceRecord, error) {
	var out []models.AttendanceRecord
	err := r.db.Preload("Enrollment.Student.User").Preload("Enrollment.AcademicSession").Preload("Enrollment.SchoolClass").Preload("Enrollment.Section").Preload("Term").Order("date DESC").Find(&out).Error
	return out, err
}
func (r *attendanceRepo) Get(id uuid.UUID) (models.AttendanceRecord, error) {
	var v models.AttendanceRecord
	err := r.db.Preload("Enrollment.Student.User").Preload("Enrollment.AcademicSession").Preload("Enrollment.SchoolClass").Preload("Enrollment.Section").Preload("Term").First(&v, "id = ?", id).Error
	return v, err
}
func (r *attendanceRepo) Update(v models.AttendanceRecord) error { return r.db.Save(&v).Error }
func (r *attendanceRepo) Delete(id uuid.UUID) error { return r.db.Delete(&models.AttendanceRecord{}, "id = ?", id).Error }
