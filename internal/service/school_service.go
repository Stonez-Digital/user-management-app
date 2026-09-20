package service

import (
    "errors"
    "strings"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/gorm"
)

var ErrSchoolNotFound = errors.New("school not found")
var ErrInvalidSchoolName = errors.New("invalid school name")

type SchoolService struct{ repo repository.SchoolRepository }

func NewSchoolService(repo repository.SchoolRepository) *SchoolService { return &SchoolService{repo: repo} }

func (s *SchoolService) GetSchool(id uuid.UUID) (models.School, error) {
    school, err := s.repo.GetByID(id)
    if errors.Is(err, gorm.ErrRecordNotFound) { return school, ErrSchoolNotFound }
    return school, err
}

func (s *SchoolService) UpdateSchool(id uuid.UUID, school models.School) error {
    if name := strings.TrimSpace(school.Name); name != "" && len(name) <= 160 {
        school.Name = name
    } else {
        return ErrInvalidSchoolName
    }
    if _, err := s.GetSchool(id); err != nil { return err }
    return s.repo.Update(id, school)
}
