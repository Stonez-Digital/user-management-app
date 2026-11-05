package service

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

var ErrSchoolEnquiryNotFound = errors.New("school enquiry not found")
var ErrSchoolEnquiryInvalidStatus = errors.New("invalid school enquiry status")

type SchoolEnquiryService struct { db *gorm.DB }

func NewSchoolEnquiryService(db *gorm.DB) *SchoolEnquiryService {
	return &SchoolEnquiryService{db: db}
}

func (s *SchoolEnquiryService) Create(schoolID uuid.UUID, enquiry *models.SchoolEnquiry) error {
	enquiry.ID = uuid.New()
	enquiry.SchoolID = schoolID
	enquiry.FullName = strings.TrimSpace(enquiry.FullName)
	enquiry.Email = strings.ToLower(strings.TrimSpace(enquiry.Email))
	enquiry.Phone = strings.TrimSpace(enquiry.Phone)
	enquiry.Subject = strings.TrimSpace(enquiry.Subject)
	enquiry.Message = strings.TrimSpace(enquiry.Message)
	enquiry.Status = models.SchoolEnquiryNew
	if enquiry.FullName == "" || enquiry.Email == "" || enquiry.Subject == "" || enquiry.Message == "" || enquiry.ConsentAt.IsZero() {
		return errors.New("required enquiry fields are missing")
	}
	return s.db.Create(enquiry).Error
}

func (s *SchoolEnquiryService) List(schoolID uuid.UUID, limit, offset int) ([]models.SchoolEnquiry, int64, error) {
	if limit < 1 || limit > 100 { limit = 50 }
	if offset < 0 { offset = 0 }
	var enquiries []models.SchoolEnquiry
	var total int64
	query := s.db.Model(&models.SchoolEnquiry{}).Where("school_id = ?", schoolID)
	if err := query.Count(&total).Error; err != nil { return nil, 0, err }
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&enquiries).Error
	return enquiries, total, err
}

func (s *SchoolEnquiryService) UpdateStatus(schoolID, enquiryID uuid.UUID, status string) (models.SchoolEnquiry, error) {
	var enquiry models.SchoolEnquiry
	if err := s.db.Where("school_id = ? AND id = ?", schoolID, enquiryID).First(&enquiry).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { return enquiry, ErrSchoolEnquiryNotFound }
		return enquiry, err
	}
	switch status {
	case models.SchoolEnquiryNew, models.SchoolEnquiryInProgress, models.SchoolEnquiryResponded, models.SchoolEnquiryClosed:
	default:
		return enquiry, ErrSchoolEnquiryInvalidStatus
	}
	if err := s.db.Model(&enquiry).Update("status", status).Error; err != nil { return enquiry, err }
	return enquiry, s.db.Where("school_id = ? AND id = ?", schoolID, enquiryID).First(&enquiry).Error
}

func (s *SchoolEnquiryService) Delete(schoolID, enquiryID uuid.UUID) error {
	result := s.db.Where("school_id = ? AND id = ?", schoolID, enquiryID).Delete(&models.SchoolEnquiry{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSchoolEnquiryNotFound
	}
	return nil
}
