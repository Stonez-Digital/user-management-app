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
    ErrClassSubjectNotFound = errors.New("class subject not found")
    ErrClassSubjectDuplicate = errors.New("class subject already exists")
    ErrClassSubjectInvalid = errors.New("invalid class subject")
)

type ClassSubjectService struct { repo repository.ClassSubjectRepository; db *gorm.DB }

func NewClassSubjectService(repo repository.ClassSubjectRepository, db *gorm.DB) *ClassSubjectService {
    return &ClassSubjectService{repo: repo, db: db}
}
func (s *ClassSubjectService) DB() *gorm.DB { return s.db }

func (s *ClassSubjectService) validate(schoolID uuid.UUID, v models.ClassSubject) (models.ClassSubject, error) {
    v.CurriculumVersion = strings.TrimSpace(v.CurriculumVersion)
    v.Category = strings.ToLower(strings.TrimSpace(v.Category))
    v.SelectionGroup = strings.TrimSpace(v.SelectionGroup)
    if v.AcademicSessionID == uuid.Nil || v.ClassID == uuid.Nil || v.SubjectID == uuid.Nil || v.CurriculumVersion == "" {
        return v, ErrClassSubjectInvalid
    }
    switch v.Category {
    case models.SubjectCategoryCore, models.SubjectCategoryOptional, models.SubjectCategoryReligious, models.SubjectCategoryTrade:
    default:
        return v, ErrClassSubjectInvalid
    }
    if v.Required && v.Category == models.SubjectCategoryOptional {
        return v, ErrClassSubjectInvalid
    }
    return v, nil
}

func (s *ClassSubjectService) validateRelationships(schoolID uuid.UUID, v models.ClassSubject) error {
    var count int64
    if err := s.db.Model(&models.AcademicSession{}).Where("id = ? AND school_id = ?", v.AcademicSessionID, schoolID).Count(&count).Error; err != nil || count != 1 { return ErrClassSubjectInvalid }
    if err := s.db.Model(&models.SchoolClass{}).Where("id = ? AND school_id = ?", v.ClassID, schoolID).Count(&count).Error; err != nil || count != 1 { return ErrClassSubjectInvalid }
    if err := s.db.Model(&models.Subject{}).Where("id = ? AND school_id = ?", v.SubjectID, schoolID).Count(&count).Error; err != nil || count != 1 { return ErrClassSubjectInvalid }
    return nil
}

func (s *ClassSubjectService) Create(schoolID uuid.UUID, v models.ClassSubject) (models.ClassSubject, error) {
    var err error
    if v, err = s.validate(schoolID, v); err != nil { return v, err }
    if err = s.validateRelationships(schoolID, v); err != nil { return v, err }
    var existing models.ClassSubject
    err = s.db.Where("school_id = ? AND academic_session_id = ? AND class_id = ? AND subject_id = ? AND curriculum_version = ?",
        schoolID, v.AcademicSessionID, v.ClassID, v.SubjectID, v.CurriculumVersion).First(&existing).Error
    if err == nil { return v, ErrClassSubjectDuplicate }
    if !errors.Is(err, gorm.ErrRecordNotFound) { return v, err }
    return s.repo.Create(schoolID, v)
}

func (s *ClassSubjectService) List(schoolID, sessionID uuid.UUID, classID *uuid.UUID) ([]models.ClassSubject, error) {
    return s.repo.List(schoolID, sessionID, classID)
}
func (s *ClassSubjectService) Get(schoolID, id uuid.UUID) (models.ClassSubject, error) {
    v, err := s.repo.Get(schoolID, id)
    if errors.Is(err, gorm.ErrRecordNotFound) { return v, ErrClassSubjectNotFound }
    return v, err
}
func (s *ClassSubjectService) Update(schoolID uuid.UUID, v models.ClassSubject) error {
    if _, err := s.Get(schoolID, v.ID); err != nil { return err }
    var err error
    if v, err = s.validate(schoolID, v); err != nil { return err }
    if err = s.validateRelationships(schoolID, v); err != nil { return err }
    var existing models.ClassSubject
    err = s.db.Where("school_id = ? AND academic_session_id = ? AND class_id = ? AND subject_id = ? AND curriculum_version = ? AND id <> ?",
        schoolID, v.AcademicSessionID, v.ClassID, v.SubjectID, v.CurriculumVersion, v.ID).First(&existing).Error
    if err == nil { return ErrClassSubjectDuplicate }
    if !errors.Is(err, gorm.ErrRecordNotFound) { return err }
    return s.repo.Update(schoolID, v)
}
func (s *ClassSubjectService) Delete(schoolID, id uuid.UUID) error {
    if _, err := s.Get(schoolID, id); err != nil { return err }
    return s.repo.Delete(schoolID, id)
}
