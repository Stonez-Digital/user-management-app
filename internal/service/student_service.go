package service

import (
    "errors"
    "strings"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/authz"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/gorm"
 )

var (
    ErrStudentNotFound = errors.New("student not found")
    ErrStudentUserNotFound = errors.New("student user not found")
    ErrStudentUserRole = errors.New("student user must have student role")
    ErrDuplicateAdmission = errors.New("admission number already exists")
    ErrDuplicateStudentUser = errors.New("user already has a student profile")
 )

type StudentService struct { repo repository.StudentRepository; db *gorm.DB }
func NewStudentService(repo repository.StudentRepository, db *gorm.DB) *StudentService { return &StudentService{repo: repo, db: db} }
func (s *StudentService) DB() *gorm.DB { return s.db }
func (s *StudentService) CreateStudent(student models.Student) (models.Student, error) {
    student.AdmissionNumber = strings.TrimSpace(student.AdmissionNumber)
    var user models.User
    if err := s.db.First(&user, "id = ?", student.UserID).Error; err != nil { if errors.Is(err, gorm.ErrRecordNotFound) { return student, ErrStudentUserNotFound }; return student, err }
    if user.Role != authz.RoleStudent { return student, ErrStudentUserRole }
    if _, err := s.repo.GetByAdmissionNumber(student.AdmissionNumber); err == nil { return student, ErrDuplicateAdmission } else if !errors.Is(err, gorm.ErrRecordNotFound) { return student, err }
    if _, err := s.repo.GetByUserID(student.UserID); err == nil { return student, ErrDuplicateStudentUser } else if !errors.Is(err, gorm.ErrRecordNotFound) { return student, err }
    return s.repo.Create(student)
}
func (s *StudentService) GetStudents() ([]models.Student, error) { return s.repo.GetAll() }
func (s *StudentService) GetStudent(id uuid.UUID) (models.Student, error) { student, err := s.repo.GetByID(id); if errors.Is(err, gorm.ErrRecordNotFound) { return student, ErrStudentNotFound }; return student, err }
func (s *StudentService) UpdateStudent(student models.Student) error {
    existing, err := s.repo.GetByID(student.ID); if errors.Is(err, gorm.ErrRecordNotFound) { return ErrStudentNotFound }; if err != nil { return err }
    if student.AdmissionNumber != existing.AdmissionNumber { if _, err := s.repo.GetByAdmissionNumber(student.AdmissionNumber); err == nil { return ErrDuplicateAdmission } else if !errors.Is(err, gorm.ErrRecordNotFound) { return err } }
    student.UserID = existing.UserID; return s.repo.Update(student)
}
func (s *StudentService) DeleteStudent(id uuid.UUID) error { if _, err := s.GetStudent(id); err != nil { return err }; return s.repo.Delete(id) }