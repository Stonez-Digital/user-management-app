package service

import (
	"errors"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/authz"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/gorm"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrAssignmentNotFound = errors.New("teacher assignment not found")
	ErrAssignmentDuplicate = errors.New("teacher assignment already exists")
	ErrAssignmentInvalid = errors.New("invalid teacher assignment")
	ErrAssignmentInUse = errors.New("teacher assignment is already used by academic records")
	ErrAssignmentConflict = errors.New("teacher allocation conflicts with an existing class-teacher allocation")
)

type TeacherAssignmentService struct { repo repository.TeacherAssignmentRepository; db *gorm.DB }

func NewTeacherAssignmentService(r repository.TeacherAssignmentRepository, db *gorm.DB) *TeacherAssignmentService { return &TeacherAssignmentService{repo:r, db:db} }
func (s *TeacherAssignmentService) DB() *gorm.DB { return s.db }

func (s *TeacherAssignmentService) validate(schoolID uuid.UUID, v models.TeacherAssignment) error {
	if v.AllocationType == "" { v.AllocationType = models.TeacherAllocationSubject }
	if v.AllocationType != models.TeacherAllocationSubject && v.AllocationType != models.TeacherAllocationClass { return ErrAssignmentInvalid }
	var teacher models.User
	if err := s.db.Where("id = ? AND school_id = ?", v.TeacherID, schoolID).First(&teacher).Error; err != nil || teacher.Role != authz.RoleTeacher || !teacher.Active { return ErrAssignmentInvalid }
	var subject models.Subject
	if err := s.db.Where("id = ? AND school_id = ?", v.SubjectID, schoolID).First(&subject).Error; err != nil || !subject.Active { return ErrAssignmentInvalid }
	var session models.AcademicSession
	if err := s.db.Where("id = ? AND school_id = ?", v.AcademicSessionID, schoolID).First(&session).Error; err != nil { return ErrAssignmentInvalid }
	var term models.Term
	if err := s.db.Where("id = ? AND school_id = ? AND academic_session_id = ?", v.TermID, schoolID, v.AcademicSessionID).First(&term).Error; err != nil { return ErrAssignmentInvalid }
	if term.StartDate.Before(session.StartDate) || term.EndDate.After(session.EndDate) { return ErrAssignmentInvalid }
	var class models.SchoolClass
	if err := s.db.Where("id = ? AND school_id = ?", v.ClassID, schoolID).First(&class).Error; err != nil { return ErrAssignmentInvalid }
	if v.SectionID != nil {
		var section models.Section
		if err := s.db.Where("id = ? AND school_id = ? AND class_id = ?", *v.SectionID, schoolID, v.ClassID).First(&section).Error; err != nil { return ErrAssignmentInvalid }
	}
	return nil
}

func (s *TeacherAssignmentService) hasConflict(schoolID uuid.UUID, v models.TeacherAssignment) (bool, error) {
	if v.AllocationType != models.TeacherAllocationClass || !v.Active { return false, nil }
	q := s.db.Where("school_id = ? AND allocation_type = ? AND active = true AND academic_session_id = ? AND term_id = ? AND class_id = ?", schoolID, models.TeacherAllocationClass, v.AcademicSessionID, v.TermID, v.ClassID)
	if v.SectionID == nil { q = q.Where("section_id IS NULL") } else { q = q.Where("section_id = ?", *v.SectionID) }
	if v.ID != uuid.Nil { q = q.Where("id <> ?", v.ID) }
	var count int64
	if err := q.Model(&models.TeacherAssignment{}).Count(&count).Error; err != nil { return false, err }
	return count > 0, nil
}

func (s *TeacherAssignmentService) Create(schoolID uuid.UUID, v models.TeacherAssignment) (models.TeacherAssignment, error) {
	v.SchoolID = schoolID
	if v.AllocationType == "" { v.AllocationType = models.TeacherAllocationSubject }
	if err := s.validate(schoolID, v); err != nil { return v, err }
	if conflict, err := s.hasConflict(schoolID, v); err != nil { return v, err } else if conflict { return v, ErrAssignmentConflict }
	var existing models.TeacherAssignment
	q := s.db.Where("school_id = ? AND teacher_id = ? AND subject_id = ? AND academic_session_id = ? AND term_id = ? AND class_id = ? AND allocation_type = ?", schoolID, v.TeacherID, v.SubjectID, v.AcademicSessionID, v.TermID, v.ClassID, v.AllocationType)
	if v.SectionID == nil { q = q.Where("section_id IS NULL") } else { q = q.Where("section_id = ?", *v.SectionID) }
	if err := q.First(&existing).Error; err == nil { return v, ErrAssignmentDuplicate } else if !errors.Is(err, gorm.ErrRecordNotFound) { return v, err }
	created, err := s.repo.Create(schoolID, v)
	if err != nil {
		if isPostgresAssignmentUniqueViolation(err, "uq_active_class_teacher_unsectioned") || isPostgresAssignmentUniqueViolation(err, "uq_active_class_teacher_sectioned") { return v, ErrAssignmentConflict }
		if isPostgresAssignmentUniqueViolation(err, "uq_teacher_assignment_unsectioned") || isPostgresAssignmentUniqueViolation(err, "uq_teacher_assignment_sectioned") { return v, ErrAssignmentDuplicate }
		return v, err
	}
	return created, nil
}

func (s *TeacherAssignmentService) List(schoolID uuid.UUID) ([]models.TeacherAssignment, error) { return s.repo.List(schoolID) }
func (s *TeacherAssignmentService) ListForTeacher(schoolID, teacherID, sessionID, termID uuid.UUID) ([]models.TeacherAssignment, error) {
    items, err := s.repo.List(schoolID)
    if err != nil { return nil, err }
    filtered := make([]models.TeacherAssignment, 0)
    for _, item := range items {
        if item.TeacherID == teacherID && item.AcademicSessionID == sessionID && item.TermID == termID { filtered = append(filtered, item) }
    }
    return filtered, nil
}
func (s *TeacherAssignmentService) Get(schoolID, id uuid.UUID) (models.TeacherAssignment, error) { v,err:=s.repo.Get(schoolID,id); if errors.Is(err,gorm.ErrRecordNotFound){return v,ErrAssignmentNotFound}; return v,err }
func (s *TeacherAssignmentService) Update(schoolID uuid.UUID, v models.TeacherAssignment) error {
	if _, err := s.Get(schoolID, v.ID); err != nil { return err }
	if v.AllocationType == "" { v.AllocationType = models.TeacherAllocationSubject }
	v.SchoolID = schoolID
	if err := s.validate(schoolID, v); err != nil { return err }
	if conflict, err := s.hasConflict(schoolID, v); err != nil { return err } else if conflict { return ErrAssignmentConflict }
	var existing models.TeacherAssignment
	q := s.db.Where("school_id = ? AND teacher_id = ? AND subject_id = ? AND academic_session_id = ? AND term_id = ? AND class_id = ? AND allocation_type = ? AND id <> ?", schoolID, v.TeacherID, v.SubjectID, v.AcademicSessionID, v.TermID, v.ClassID, v.AllocationType, v.ID)
	if v.SectionID == nil { q = q.Where("section_id IS NULL") } else { q = q.Where("section_id = ?", *v.SectionID) }
	if err := q.First(&existing).Error; err == nil { return ErrAssignmentDuplicate } else if !errors.Is(err, gorm.ErrRecordNotFound) { return err }
	if err:=s.repo.Update(schoolID, v);err!=nil {
	if isPostgresAssignmentUniqueViolation(err, "uq_active_class_teacher_unsectioned") || isPostgresAssignmentUniqueViolation(err, "uq_active_class_teacher_sectioned") { return ErrAssignmentConflict }
	if isPostgresAssignmentUniqueViolation(err, "uq_teacher_assignment_unsectioned") || isPostgresAssignmentUniqueViolation(err, "uq_teacher_assignment_sectioned") { return ErrAssignmentDuplicate }
	return err
}
return nil
}
func (s *TeacherAssignmentService) Delete(schoolID,id uuid.UUID) error {
	if _, err := s.Get(schoolID, id); err != nil { return err }
	var assessmentCount, timetableCount int64
	if err := s.db.Model(&models.Assessment{}).Where("school_id = ? AND teacher_assignment_id = ?", schoolID, id).Count(&assessmentCount).Error; err != nil { return err }
	if err := s.db.Model(&models.TimetableEntry{}).Where("school_id = ? AND teacher_assignment_id = ?", schoolID, id).Count(&timetableCount).Error; err != nil { return err }
	if assessmentCount > 0 || timetableCount > 0 { return ErrAssignmentInUse }
	return s.repo.Delete(schoolID, id)
}


func (s *TeacherAssignmentService) Coverage(schoolID, sessionID, termID uuid.UUID) ([]models.TeacherAssignment, error) {
	items, err := s.repo.List(schoolID)
	if err != nil { return nil, err }
	out := make([]models.TeacherAssignment, 0, len(items))
	for _, item := range items {
		if sessionID != uuid.Nil && item.AcademicSessionID != sessionID { continue }
		if termID != uuid.Nil && item.TermID != termID { continue }
		out = append(out, item)
	}
	return out, nil
}

func isPostgresAssignmentUniqueViolation(err error,constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err,&pgErr) && pgErr.Code=="23505" && pgErr.ConstraintName==constraint
}
