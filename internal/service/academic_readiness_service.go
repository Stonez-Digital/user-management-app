package service

import (
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type AcademicReadiness struct {
    SchoolName string `json:"school_name"`
    SchoolStatus string `json:"school_status"`
    ActiveSession *models.AcademicSession `json:"active_session,omitempty"`
    ActiveTerm *models.Term `json:"active_term,omitempty"`
    Classes int64 `json:"classes"`
    Sections int64 `json:"sections"`
    Subjects int64 `json:"subjects"`
    TeacherAssignments int64 `json:"teacher_assignments"`
    Checks map[string]bool `json:"checks"`
    Ready bool `json:"ready"`
}

type AcademicReadinessService struct { db *gorm.DB }

func NewAcademicReadinessService(db *gorm.DB) *AcademicReadinessService { return &AcademicReadinessService{db: db} }

func (s *AcademicReadinessService) Get(schoolID uuid.UUID) (AcademicReadiness, error) {
    var school models.School
    if err := s.db.Select("id","name","status").First(&school, "id = ?", schoolID).Error; err != nil { return AcademicReadiness{}, err }

    var session models.AcademicSession
    sessionFound := s.db.Where("school_id = ? AND status = ?", schoolID, models.AcademicStatusActive).Order("start_date DESC").First(&session).Error == nil

    var term models.Term
    termFound := false
    if sessionFound {
        termFound = s.db.Where("school_id = ? AND academic_session_id = ? AND status = ?", schoolID, session.ID, models.AcademicStatusActive).Order("start_date DESC").First(&term).Error == nil
    }

    var classes, sections, subjects, assignments int64
    if err := s.db.Model(&models.SchoolClass{}).Where("school_id = ?", schoolID).Count(&classes).Error; err != nil { return AcademicReadiness{}, err }
    if err := s.db.Model(&models.Section{}).Where("school_id = ?", schoolID).Count(&sections).Error; err != nil { return AcademicReadiness{}, err }
    if err := s.db.Model(&models.Subject{}).Where("school_id = ? AND active = ?", schoolID, true).Count(&subjects).Error; err != nil { return AcademicReadiness{}, err }
    assignmentQuery := s.db.Model(&models.TeacherAssignment{}).Where("school_id = ? AND active = ?", schoolID, true)
    if sessionFound { assignmentQuery = assignmentQuery.Where("academic_session_id = ?", session.ID) }
    if termFound { assignmentQuery = assignmentQuery.Where("term_id = ?", term.ID) }
    if err := assignmentQuery.Count(&assignments).Error; err != nil { return AcademicReadiness{}, err }

    checks := map[string]bool{
        "school_profile": school.Status == models.SchoolStatusActive && school.Name != "",
        "active_session": sessionFound,
        "active_term": termFound,
        "classes": classes > 0,
        "subjects": subjects > 0,
        "teacher_coverage": assignments > 0,
    }
    ready := true
    for _, ok := range checks { if !ok { ready = false; break } }

    var activeSession *models.AcademicSession
    if sessionFound { activeSession = &session }
    var activeTerm *models.Term
    if termFound { activeTerm = &term }

    return AcademicReadiness{
        SchoolName: school.Name,
        SchoolStatus: school.Status,
        ActiveSession: activeSession,
        ActiveTerm: activeTerm,
        Classes: classes,
        Sections: sections,
        Subjects: subjects,
        TeacherAssignments: assignments,
        Checks: checks,
        Ready: ready,
    }, nil
}
