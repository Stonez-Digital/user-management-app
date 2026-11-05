package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func enquiryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.School{}, &models.SchoolEnquiry{}); err != nil { t.Fatal(err) }
	return db
}

func TestSchoolEnquiriesAreTenantScoped(t *testing.T) {
	db := enquiryTestDB(t)
	svc := NewSchoolEnquiryService(db)
	a := models.School{Name:"School A", Code:"ENQ-A"}
	b := models.School{Name:"School B", Code:"ENQ-B"}
	if err := db.Create(&a).Error; err != nil { t.Fatal(err) }
	if err := db.Create(&b).Error; err != nil { t.Fatal(err) }

	first := &models.SchoolEnquiry{FullName:"  Parent One ", Email:" PARENT@EXAMPLE.COM ", Subject:"Admissions", Message:"Please share admission details.", ConsentAt:time.Now().UTC()}
	if err := svc.Create(a.ID, first); err != nil { t.Fatal(err) }
	if first.SchoolID != a.ID || first.Status != models.SchoolEnquiryNew || first.Email != "parent@example.com" { t.Fatalf("enquiry normalization or ownership failed: %+v", first) }

	items, total, err := svc.List(a.ID, 50, 0)
	if err != nil || total != 1 || len(items) != 1 { t.Fatalf("expected one school A enquiry, got %d/%d (%v)", len(items), total, err) }
	other, otherTotal, err := svc.List(b.ID, 50, 0)
	if err != nil || otherTotal != 0 || len(other) != 0 { t.Fatalf("school B must not see school A enquiries, got %d/%d (%v)", len(other), otherTotal, err) }

	if _, err := svc.UpdateStatus(b.ID, first.ID, models.SchoolEnquiryResponded); err != ErrSchoolEnquiryNotFound { t.Fatalf("cross-school update should look not found, got %v", err) }
	updated, err := svc.UpdateStatus(a.ID, first.ID, models.SchoolEnquiryInProgress)
	if err != nil || updated.Status != models.SchoolEnquiryInProgress { t.Fatalf("expected status update, got %+v (%v)", updated, err) }
	if _, err := svc.UpdateStatus(a.ID, first.ID, "approved"); err != ErrSchoolEnquiryInvalidStatus { t.Fatalf("expected invalid status error, got %v", err) }

	if err := svc.Delete(b.ID, first.ID); err != ErrSchoolEnquiryNotFound {
		t.Fatalf("cross-school delete should look not found, got %v", err)
	}
	if err := svc.Delete(a.ID, first.ID); err != nil {
		t.Fatalf("school A should be able to delete its own enquiry: %v", err)
	}
	var remaining int64
	if err := db.Model(&models.SchoolEnquiry{}).Where("id = ?", first.ID).Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("expected enquiry to be deleted, found %d rows", remaining)
	}
}

func TestSchoolEnquiryRequiresConsentAndMessage(t *testing.T) {
	db := enquiryTestDB(t)
	svc := NewSchoolEnquiryService(db)
	schoolID := uuid.New()
	err := svc.Create(schoolID, &models.SchoolEnquiry{FullName:"Parent", Email:"parent@example.com", Subject:"Admissions", Message:"Hello"})
	if err == nil { t.Fatal("expected missing privacy consent to be rejected") }
}
