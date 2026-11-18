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
var ErrInvalidSchoolLogo = errors.New("invalid school logo URL")

type SchoolService struct {
    repo repository.SchoolRepository
    db   *gorm.DB
}

func NewSchoolService(repo repository.SchoolRepository, db *gorm.DB) *SchoolService {
    return &SchoolService{repo: repo, db: db}
}

func (s *SchoolService) DB() *gorm.DB { return s.db }

func (s *SchoolService) GetSchool(id uuid.UUID) (models.School, error) {
    school, err := s.repo.GetByID(id)
    if errors.Is(err, gorm.ErrRecordNotFound) { return school, ErrSchoolNotFound }
    return school, err
}

func (s *SchoolService) UpdateSchool(id uuid.UUID, school models.School) error {
    name := strings.TrimSpace(school.Name)
    if name == "" || len(name) > 160 { return ErrInvalidSchoolName }
    school.Name = name
    school.LogoURL = strings.TrimSpace(school.LogoURL)
    school.Description = strings.TrimSpace(school.Description)
    school.Address = strings.TrimSpace(school.Address)
    school.ContactEmail = strings.TrimSpace(school.ContactEmail)
    school.ContactPhone = strings.TrimSpace(school.ContactPhone)
    school.WebsiteURL = strings.TrimSpace(school.WebsiteURL)
    if len(school.LogoURL) > 1000 || len(school.Description)>1000 || len(school.Address)>300 || len(school.ContactEmail)>255 || len(school.ContactPhone)>80 || len(school.WebsiteURL)>500 { return ErrInvalidSchoolName }
    if _, err := s.GetSchool(id); err != nil { return err }
    return s.repo.Update(id, school)
}

 // UpdateSchoolLogo persists only the logo field so logo uploads do not need to
 // resubmit (or accidentally overwrite) the rest of the school profile.
 func (s *SchoolService) UpdateSchoolLogo(id uuid.UUID, logoURL string) error {
     logoURL = strings.TrimSpace(logoURL)
     if logoURL == "" || len(logoURL) > 1000 { return ErrInvalidSchoolLogo }
     if _, err := s.GetSchool(id); err != nil { return err }
     result := s.db.Model(&models.School{}).Where("id = ?", id).Update("logo_url", logoURL)
     if result.Error != nil { return result.Error }
     if result.RowsAffected != 1 { return ErrSchoolNotFound }
     return nil
 }
