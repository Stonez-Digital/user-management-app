package controller

import (
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

func newDomainReconciliationTestDB(t *testing.T) (*gorm.DB, models.School) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&models.School{}); err != nil { t.Fatal(err) }
	school := models.School{ID:uuid.New(),Name:"Legacy School",Code:"LEGACY",Slug:"legacy school'",Status:models.SchoolStatusActive}
	if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
	return db, school
}

func TestSchoolDomainReconciliationRejectsInvalidSlugBeforeCloudflare(t *testing.T) {
	db, school := newDomainReconciliationTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/platform/schools/:id/domain-reconciliation", NewSchoolDomainReconciliationController(db).Reconcile)
	req := httptest.NewRequest(http.MethodPut, "/platform/schools/"+school.ID.String()+"/domain-reconciliation", strings.NewReader(`{"slug":"bad slug","confirm":false}`))
	req.Header.Set("Content-Type","application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w,req)
	if w.Code != http.StatusBadRequest { t.Fatalf("expected 400, got %d: %s",w.Code,w.Body.String()) }
}

func TestSchoolDomainReconciliationRejectsSlugCollision(t *testing.T) {
	db, school := newDomainReconciliationTestDB(t)
	other := models.School{ID:uuid.New(),Name:"Other School",Code:"OTHER",Slug:"taken",Status:models.SchoolStatusActive}
	if err := db.Create(&other).Error; err != nil { t.Fatal(err) }
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/platform/schools/:id/domain-reconciliation", NewSchoolDomainReconciliationController(db).Reconcile)
	req := httptest.NewRequest(http.MethodPut, "/platform/schools/"+school.ID.String()+"/domain-reconciliation", strings.NewReader(`{"slug":"taken","confirm":false}`))
	req.Header.Set("Content-Type","application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w,req)
	if w.Code != http.StatusConflict { t.Fatalf("expected 409, got %d: %s",w.Code,w.Body.String()) }
}

func TestSchoolDomainReconciliationRejectsUnrecordedPreviousHostname(t *testing.T) {
	db, school := newDomainReconciliationTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/platform/schools/:id/domain-reconciliation", NewSchoolDomainReconciliationController(db).Reconcile)
	req := httptest.NewRequest(http.MethodPut, "/platform/schools/"+school.ID.String()+"/domain-reconciliation", strings.NewReader(`{"slug":"legacy-school","previous_hostname":"another-school.stonezdigital.com.ng","confirm":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict { t.Fatalf("expected 409 for unrecorded previous hostname, got %d: %s", w.Code, w.Body.String()) }
}
