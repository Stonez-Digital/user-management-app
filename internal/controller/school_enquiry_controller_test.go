package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPublicSchoolEnquiryValidatesAndPersistsOnlyConsentedMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:public-school-enquiry-controller?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.School{}, &models.SchoolEnquiry{}); err != nil { t.Fatal(err) }
	school := models.School{Name:"Enquiry Test School", Code:"ENQUIRY-TEST", Slug:"enquiry-test", Status:models.SchoolStatusActive}
	if err := db.Create(&school).Error; err != nil { t.Fatal(err) }

	ctrl := NewSchoolContentController(nil, nil, db)
	router := gin.New()
	router.POST("/public/schools/:slug/enquiries", ctrl.CreatePublicEnquiry)

	submit := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/public/schools/enquiry-test/enquiries", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder
	}

	valid := `{"full_name":"Parent One","email":"parent@example.com","subject":"Admissions","message":"Please share admission information.","consent":true}`
	if response := submit(valid); response.Code != http.StatusAccepted {
		t.Fatalf("expected accepted enquiry, got %d: %s", response.Code, response.Body.String())
	}
	var count int64
	if err := db.Model(&models.SchoolEnquiry{}).Where("school_id = ?", school.ID).Count(&count).Error; err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("expected one persisted enquiry, got %d", count) }

	bot := `{"full_name":"Bot","email":"bot@example.com","subject":"Admissions","message":"Automated enquiry message.","consent":true,"website":"https://spam.example"}`
	if response := submit(bot); response.Code != http.StatusAccepted {
		t.Fatalf("honeypot should return an indistinguishable accepted response, got %d", response.Code)
	}
	if err := db.Model(&models.SchoolEnquiry{}).Where("school_id = ?", school.ID).Count(&count).Error; err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("honeypot submission must not be stored, got %d rows", count) }

	noConsent := `{"full_name":"Parent Two","email":"parent2@example.com","subject":"General","message":"I have a question for the school.","consent":false}`
	if response := submit(noConsent); response.Code != http.StatusBadRequest {
		t.Fatalf("expected consent rejection, got %d: %s", response.Code, response.Body.String())
	}
	invalidEmail := `{"full_name":"Parent Three","email":"not-an-email","subject":"Other","message":"I have a question for the school.","consent":true}`
	if response := submit(invalidEmail); response.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid email rejection, got %d: %s", response.Code, response.Body.String())
	}
}
