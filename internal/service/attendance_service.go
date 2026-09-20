package service

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrAttendanceNotFound       = errors.New("attendance record not found")
	ErrAttendanceEnrollmentMissing = errors.New("enrollment not found")
	ErrAttendanceTermMissing    = errors.New("term not found")
	ErrAttendanceTermMismatch   = errors.New("term does not belong to enrollment academic session")
	ErrAttendanceDuplicate      = errors.New("attendance already recorded for enrollment on date")
	ErrAttendanceInvalidStatus  = errors.New("invalid attendance status")
)

type AttendanceService struct {
	repo repository.AttendanceRepository
	db   *gorm.DB
}

func NewAttendanceService(repo repository.AttendanceRepository, db *gorm.DB) *AttendanceService {
	return &AttendanceService{repo: repo, db: db}
}
func (s *AttendanceService) DB() *gorm.DB { return s.db }

func validAttendanceStatus(status string) bool {
	switch status {
	case models.AttendancePresent, models.AttendanceAbsent, models.AttendanceLate, models.AttendanceExcused:
		return true
	default:
		return false
	}
}

func (s *AttendanceService) validate(v models.AttendanceRecord) error {
	if v.Date.IsZero() { return errors.New("attendance date is required") }
	var enrollment models.StudentEnrollment
	if err := s.db.First(&enrollment, "id = ?", v.EnrollmentID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrAttendanceEnrollmentMissing
	} else if err != nil {
		return err
	}
	if enrollment.Status != models.EnrollmentStatusActive {
		return ErrAttendanceEnrollmentMissing
	}

	var term models.Term
	if err := s.db.First(&term, "id = ?", v.TermID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrAttendanceTermMissing
	} else if err != nil {
		return err
	}
	if term.AcademicSessionID != enrollment.AcademicSessionID {
		return ErrAttendanceTermMismatch
	}

	v.Status = strings.ToLower(strings.TrimSpace(v.Status))
	if !validAttendanceStatus(v.Status) {
		return ErrAttendanceInvalidStatus
	}
	return nil
}

func (s *AttendanceService) Create(v models.AttendanceRecord) (models.AttendanceRecord, error) {
	if v.Date.IsZero() {
		return v, errors.New("attendance date is required")
	}
	v.Date = time.Date(v.Date.Year(), v.Date.Month(), v.Date.Day(), 0, 0, 0, 0, time.UTC)
	if err := s.validate(v); err != nil {
		return v, err
	}
	var existing models.AttendanceRecord
	err := s.db.Where("enrollment_id = ? AND date = ?", v.EnrollmentID, v.Date).First(&existing).Error
	if err == nil {
		return v, ErrAttendanceDuplicate
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return v, err
	}
	v.Status = strings.ToLower(strings.TrimSpace(v.Status))
	return s.repo.Create(v)
}

func (s *AttendanceService) List() ([]models.AttendanceRecord, error) { return s.repo.List() }

func (s *AttendanceService) Get(id uuid.UUID) (models.AttendanceRecord, error) {
	v, err := s.repo.Get(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return v, ErrAttendanceNotFound
	}
	return v, err
}

func (s *AttendanceService) Update(v models.AttendanceRecord) error {
	current, err := s.Get(v.ID)
	if err != nil { return err }
	if err := s.validate(v); err != nil { return err }
	v.Date = time.Date(v.Date.Year(), v.Date.Month(), v.Date.Day(), 0, 0, 0, 0, time.UTC)
	if current.EnrollmentID != v.EnrollmentID || !current.Date.Equal(v.Date) {
		var duplicate models.AttendanceRecord
		if err := s.db.Where("enrollment_id = ? AND date = ? AND id <> ?", v.EnrollmentID, v.Date, v.ID).First(&duplicate).Error; err == nil {
			return ErrAttendanceDuplicate
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	v.Status = strings.ToLower(strings.TrimSpace(v.Status))
	return s.repo.Update(v)
}

func (s *AttendanceService) Delete(id uuid.UUID) error {
	if _, err := s.Get(id); err != nil { return err }
	return s.repo.Delete(id)
}
