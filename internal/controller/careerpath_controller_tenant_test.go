package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func careerPathControllerTestDB(t *testing.T) (*gorm.DB, uuid.UUID, uuid.UUID) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:careerpath-controller-"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	var school models.School
	if err := db.Where("code = ?", "DEFAULT").First(&school).Error; err != nil {
		t.Fatal(err)
	}
	schoolID := school.ID
	user := models.User{
		SchoolID: &schoolID,
		Name:     "CareerPath Route Test",
		Email:    "careerpath-route-" + uuid.NewString() + "@example.test",
		Role:     "student",
		Active:   true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	student := models.Student{
		SchoolID:       schoolID,
		UserID:         user.ID,
		AdmissionNumber: "CP-" + uuid.NewString()[:8],
	}
	if err := db.Create(&student).Error; err != nil {
		t.Fatal(err)
	}
	return db, schoolID, user.ID
}

func TestCareerPathRoutesRequireAndUseTenantTransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, schoolID, userID := careerPathControllerTestDB(t)
	ctrl := NewCareerPathController(service.NewCareerPathService(db))

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID.String())
		c.Set(middleware.SchoolIDKey, schoolID.String())
		c.Next()
	})
	router.GET("/student/careerpath/assessment", middleware.TenantTransaction(db), ctrl.Assessment)
	router.POST("/student/careerpath/attempts", middleware.TenantTransaction(db), ctrl.CreateOrResume)
	router.GET("/student/careerpath/attempts/:id", middleware.TenantTransaction(db), ctrl.Get)
	router.PUT("/student/careerpath/attempts/:id/responses", middleware.TenantTransaction(db), ctrl.SaveResponses)
	router.POST("/student/careerpath/attempts/:id/submit", middleware.TenantTransaction(db), ctrl.Submit)

	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		router.ServeHTTP(recorder, req)
		return recorder
	}

	if got := request(http.MethodGet, "/student/careerpath/assessment", ""); got.Code != http.StatusOK {
		t.Fatalf("assessment route: status=%d body=%s", got.Code, got.Body.String())
	}

	created := request(http.MethodPost, "/student/careerpath/attempts", "")
	if created.Code != http.StatusOK {
		t.Fatalf("create/resume route: status=%d body=%s", created.Code, created.Body.String())
	}
	var attempt struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &attempt); err != nil || attempt.ID == "" {
		t.Fatalf("create/resume did not return an attempt id: id=%q err=%v body=%s", attempt.ID, err, created.Body.String())
	}

	responses := map[string][]string{
		"interests":              {"build", "investigate", "create"},
		"personality_strengths":  {"curious", "creative_thinker"},
		"work_style":             {"independent", "studio"},
		"workplace_preferences":  {"behind_scenes", "digital"},
		"subjects":               {"maths", "computing", "arts"},
		"education_experience":   {"school_project", "digital_project"},
		"goals":                  {"solve", "make"},
	}
	payload, err := json.Marshal(map[string]interface{}{"responses": responses})
	if err != nil {
		t.Fatal(err)
	}
	path := "/student/careerpath/attempts/" + attempt.ID
	if got := request(http.MethodPut, path+"/responses", string(payload)); got.Code != http.StatusOK {
		t.Fatalf("save responses route: status=%d body=%s", got.Code, got.Body.String())
	}
	if got := request(http.MethodGet, path, ""); got.Code != http.StatusOK {
		t.Fatalf("get attempt route: status=%d body=%s", got.Code, got.Body.String())
	}
	if got := request(http.MethodPost, path+"/submit", ""); got.Code != http.StatusOK {
		t.Fatalf("submit route: status=%d body=%s", got.Code, got.Body.String())
	}
}

func TestCareerPathMutationFailsClosedWithoutTenantTransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, schoolID, userID := careerPathControllerTestDB(t)
	ctrl := NewCareerPathController(service.NewCareerPathService(db))
	router := gin.New()
	router.POST("/student/careerpath/attempts", func(c *gin.Context) {
		c.Set("user_id", userID.String())
		c.Set(middleware.SchoolIDKey, schoolID.String())
		c.Next()
	}, ctrl.CreateOrResume)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/student/careerpath/attempts", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected missing tenant transaction to fail closed with 500, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("tenant_transaction_required")) {
		t.Fatalf("expected tenant transaction error, got %s", recorder.Body.String())
	}
}
