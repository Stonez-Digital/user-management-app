package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeSchoolDomainProvisioner struct {
	hostname string
	err error
}

func (f *fakeSchoolDomainProvisioner) Attach(_ context.Context, hostname string) error {
	f.hostname = hostname
	return f.err
}

func TestUniqueSchoolSlugUsesNameAndAddsCodeOnlyOnCollision(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.School{}); err != nil { t.Fatal(err) }
	first, err := uniqueSchoolSlug(db, "Greenfield Academy", "GREENFIELD", uuid.Nil)
	if err != nil { t.Fatal(err) }
	if first != "greenfield-academy" { t.Fatalf("expected name-only slug, got %q", first) }
	if err := db.Create(&models.School{Name:"Greenfield Academy",Code:"GREENFIELD",Slug:first,Status:models.SchoolStatusPending}).Error; err != nil { t.Fatal(err) }
	second, err := uniqueSchoolSlug(db, "Greenfield Academy", "GREENFIELD-2", uuid.Nil)
	if err != nil { t.Fatal(err) }
	if second != "greenfield-academy-greenfield-2" { t.Fatalf("expected collision-safe suffix, got %q", second) }
}

func TestPlatformSchoolActivationAttachesExactDomainBeforeActivation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.AuditLog{}); err != nil { t.Fatal(err) }
	school := models.School{ID:uuid.New(),Name:"Bedrock International",Code:"BEDROCK",Slug:"bedrock-international",Status:models.SchoolStatusPending}
	if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
	provisioner := &fakeSchoolDomainProvisioner{}
	controller := &PlatformSchoolController{DB:db,DomainProvisioner:provisioner}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context){c.Set("user_id",uuid.New().String());c.Next()})
	router.PUT("/platform/schools/:id",controller.Update)
	req := httptest.NewRequest(http.MethodPut,"/platform/schools/"+school.ID.String(),strings.NewReader("{\"status\":\"active\"}"))
	req.Header.Set("Content-Type","application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec,req)
	if rec.Code != http.StatusOK { t.Fatalf("expected 200, got %d: %s",rec.Code,rec.Body.String()) }
	if provisioner.hostname != "bedrock-international.stonezdigital.com.ng" { t.Fatalf("unexpected provisioned hostname %q",provisioner.hostname) }
	if err := db.First(&school,"id = ?",school.ID).Error; err != nil { t.Fatal(err) }
	if school.Status != models.SchoolStatusActive { t.Fatalf("expected active status, got %q",school.Status) }
}

func TestPlatformSchoolActivationFailsClosedWhenProvisioningFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.AuditLog{}); err != nil { t.Fatal(err) }
	school := models.School{ID:uuid.New(),Name:"Bedrock International",Code:"BEDROCK",Slug:"bedrock-international",Status:models.SchoolStatusPending}
	if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
	provisioner := &fakeSchoolDomainProvisioner{err:errors.New("Cloudflare unavailable")}
	controller := &PlatformSchoolController{DB:db,DomainProvisioner:provisioner}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/platform/schools/:id",controller.Update)
	req := httptest.NewRequest(http.MethodPut,"/platform/schools/"+school.ID.String(),strings.NewReader("{\"status\":\"active\"}"))
	req.Header.Set("Content-Type","application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec,req)
	if rec.Code != http.StatusBadGateway { t.Fatalf("expected 502, got %d: %s",rec.Code,rec.Body.String()) }
	if err := db.First(&school,"id = ?",school.ID).Error; err != nil { t.Fatal(err) }
	if school.Status != models.SchoolStatusPending { t.Fatalf("school must remain pending after provisioning failure, got %q",school.Status) }
}
