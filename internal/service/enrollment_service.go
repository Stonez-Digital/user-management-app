package service

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrEnrollmentNotFound      = errors.New("enrollment not found")
	ErrEnrollmentDuplicate     = errors.New("student already enrolled in academic session")
	ErrEnrollmentStudentMissing = errors.New("student not found")
	ErrEnrollmentSessionMissing = errors.New("academic session not found")
	ErrEnrollmentClassMissing   = errors.New("class not found")
	ErrEnrollmentSectionMissing = errors.New("section not found")
	ErrEnrollmentSectionMismatch = errors.New("section does not belong to class")
	ErrEnrollmentInvalidStatus = errors.New("invalid enrollment status")
)

type EnrollmentService struct {
	repo repository.EnrollmentRepository
	db   *gorm.DB
}

func NewEnrollmentService(repo repository.EnrollmentRepository, db *gorm.DB) *EnrollmentService {
	return &EnrollmentService{repo: repo, db: db}
}
func (s *EnrollmentService) DB() *gorm.DB { return s.db }

func (s *EnrollmentService) Create(v models.StudentEnrollment) (models.StudentEnrollment, error) {
	var student models.Student
	if err := s.db.First(&student, "id = ?", v.StudentID).Error; errors.Is(err, gorm.ErrRecordNotFound) { return v, ErrEnrollmentStudentMissing } else if err != nil { return v, err }

	var session models.AcademicSession
	if err := s.db.First(&session, "id = ?", v.AcademicSessionID).Error; errors.Is(err, gorm.ErrRecordNotFound) { return v, ErrEnrollmentSessionMissing } else if err != nil { return v, err }

	var class models.SchoolClass
	if err := s.db.First(&class, "id = ?", v.ClassID).Error; errors.Is(err, gorm.ErrRecordNotFound) { return v, ErrEnrollmentClassMissing } else if err != nil { return v, err }

	var section models.Section
	if err := s.db.First(&section, "id = ?", v.SectionID).Error; errors.Is(err, gorm.ErrRecordNotFound) { return v, ErrEnrollmentSectionMissing } else if err != nil { return v, err }
	if section.ClassID != v.ClassID { return v, ErrEnrollmentSectionMismatch }

	v.Status = strings.ToLower(strings.TrimSpace(v.Status))
	if v.Status == "" { v.Status = models.EnrollmentStatusActive }
	if v.Status != models.EnrollmentStatusActive && v.Status != models.EnrollmentStatusCompleted && v.Status != models.EnrollmentStatusWithdrawn { return v, ErrEnrollmentInvalidStatus }

	var existing models.StudentEnrollment
	err := s.db.Where("student_id = ? AND academic_session_id = ?", v.StudentID, v.AcademicSessionID).First(&existing).Error
	if err == nil { return v, ErrEnrollmentDuplicate }
	if !errors.Is(err, gorm.ErrRecordNotFound) { return v, err }

	return s.repo.Create(v)
}

func (s *EnrollmentService) List() ([]models.StudentEnrollment, error) { return s.repo.List() }
func (s *EnrollmentService) Get(id uuid.UUID) (models.StudentEnrollment, error) {
	v, err := s.repo.Get(id)
	if errors.Is(err, gorm.ErrRecordNotFound) { return v, ErrEnrollmentNotFound }
	return v, err
}
func (s *EnrollmentService) Update(v models.StudentEnrollment) error {
	current, err := s.Get(v.ID)
	if err != nil { return err }
	if v.Status != models.EnrollmentStatusActive && v.Status != models.EnrollmentStatusCompleted && v.Status != models.EnrollmentStatusWithdrawn { return ErrEnrollmentInvalidStatus }
	if _, err := s.db.First(&models.Student{}, "id = ?", v.StudentID).RowsAffected, error(nil); err != nil { return err }
	var student models.Student
	if err := s.db.First(&student, "id = ?", v.StudentID).Error; errors.Is(err, gorm.ErrRecordNotFound) { return ErrEnrollmentStudentMissing } else if err != nil { return err }
	var session models.AcademicSession
	if err := s.db.First(&session, "id = ?", v.AcademicSessionID).Error; errors.Is(err, gorm.ErrRecordNotFound) { return ErrEnrollmentSessionMissing } else if err != nil { return err }
	var class models.SchoolClass
	if err := s.db.First(&class, "id = ?", v.ClassID).Error; errors.Is(err, gorm.ErrRecordNotFound) { return ErrEnrollmentClassMissing } else if err != nil { return err }
	var section models.Section
	if err := s.db.First(&section, "id = ?", v.SectionID).Error; errors.Is(err, gorm.ErrRecordNotFound) { return ErrEnrollmentSectionMissing } else if err != nil { return err }
	if section.ClassID != v.ClassID { return ErrEnrollmentSectionMismatch }
	if current.StudentID != v.StudentID || current.AcademicSessionID != v.AcademicSessionID {
		var duplicate models.StudentEnrollment
		if err := s.db.Where("student_id = ? AND academic_session_id = ? AND id <> ?", v.StudentID, v.AcademicSessionID, v.ID).First(&duplicate).Error; err == nil { return ErrEnrollmentDuplicate } else if !errors.Is(err, gorm.ErrRecordNotFound) { return err }
	}
	return s.repo.Update(v)
}
func (s *EnrollmentService) Delete(id uuid.UUID) error {
	if _, err := s.Get(id); err != nil { return err }
	return s.repo.Delete(id)
}
