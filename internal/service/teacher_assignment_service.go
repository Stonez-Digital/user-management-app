package service

import (
	"errors"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/authz"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrAssignmentNotFound = errors.New("teacher assignment not found")
	ErrAssignmentDuplicate = errors.New("teacher assignment already exists")
	ErrAssignmentInvalid = errors.New("invalid teacher assignment")
)

type TeacherAssignmentService struct { repo repository.TeacherAssignmentRepository; db *gorm.DB }

func NewTeacherAssignmentService(r repository.TeacherAssignmentRepository, db *gorm.DB) *TeacherAssignmentService { return &TeacherAssignmentService{repo:r, db:db} }
func (s *TeacherAssignmentService) DB() *gorm.DB { return s.db }

func (s *TeacherAssignmentService) validate(schoolID uuid.UUID, v models.TeacherAssignment) error {
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

func (s *TeacherAssignmentService) Create(schoolID uuid.UUID, v models.TeacherAssignment) (models.TeacherAssignment, error) {
	v.SchoolID = schoolID
	if err := s.validate(schoolID, v); err != nil { return v, err }
	var existing models.TeacherAssignment
	q := s.db.Where("school_id = ? AND teacher_id = ? AND subject_id = ? AND academic_session_id = ? AND term_id = ? AND class_id = ?", schoolID, v.TeacherID, v.SubjectID, v.AcademicSessionID, v.TermID, v.ClassID)
	if v.SectionID == nil { q = q.Where("section_id IS NULL") } else { q = q.Where("section_id = ?", *v.SectionID) }
	if err := q.First(&existing).Error; err == nil { return v, ErrAssignmentDuplicate } else if !errors.Is(err, gorm.ErrRecordNotFound) { return v, err }
	return s.repo.Create(schoolID, v)
}

func (s *TeacherAssignmentService) List(schoolID uuid.UUID) ([]models.TeacherAssignment, error) { return s.repo.List(schoolID) }
func (s *TeacherAssignmentService) Get(schoolID, id uuid.UUID) (models.TeacherAssignment, error) { v,err:=s.repo.Get(schoolID,id); if errors.Is(err,gorm.ErrRecordNotFound){return v,ErrAssignmentNotFound}; return v,err }
func (s *TeacherAssignmentService) Update(schoolID uuid.UUID, v models.TeacherAssignment) error { if _,err:=s.Get(schoolID,v.ID);err!=nil{return err};v.SchoolID=schoolID;if err:=s.validate(schoolID,v);err!=nil{return err};return s.repo.Update(schoolID,v) }
func (s *TeacherAssignmentService) Delete(schoolID,id uuid.UUID) error { if _,err:=s.Get(schoolID,id);err!=nil{return err};return s.repo.Delete(schoolID,id) }
