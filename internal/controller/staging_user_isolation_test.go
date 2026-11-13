package controller

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/onoja217/users-management-app/internal/authz"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"github.com/onoja217/users-management-app/internal/service"
)


func tenantContextTransactionRouter(db *gorm.DB, handler gin.HandlerFunc, schoolID, userID, method, path, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.SchoolIDKey, schoolID)
		c.Set(middleware.UserIDKey, userID)
		if id := strings.TrimPrefix(path, "/admin/users/"); id != path {
			c.Params = gin.Params{{Key: "id", Value: id}}
		}
		c.Next()
	})
	r.Use(middleware.TenantTransaction(db))
	r.Handle(method, path, handler)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestIsolatedStagingAcceptanceSchoolUserReadAndProvisioningIsolation(t *testing.T) {
	db := stagingAcceptanceDB(t)

	schoolA := models.School{Name: "Staging Read Academy A", Code: "READ-A", Status: models.SchoolStatusActive}
	schoolB := models.School{Name: "Staging Read Academy B", Code: "READ-B", Status: models.SchoolStatusActive}
	if err := db.Create(&schoolA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&schoolB).Error; err != nil {
		t.Fatal(err)
	}

	adminA := models.User{Name: "Read Admin A", Email: "read-admin-a@staging.example", Role: authz.RoleSchoolAdmin, Active: true, SchoolID: &schoolA.ID}
	adminB := models.User{Name: "Read Admin B", Email: "read-admin-b@staging.example", Role: authz.RoleSchoolAdmin, Active: true, SchoolID: &schoolB.ID}
	studentA := models.User{Name: "Read Student A", Email: "read-student-a@staging.example", Role: authz.RoleStudent, Active: true, SchoolID: &schoolA.ID}
	studentB := models.User{Name: "Read Student B", Email: "read-student-b@staging.example", Role: authz.RoleStudent, Active: true, SchoolID: &schoolB.ID}
	for _, user := range []*models.User{&adminA, &adminB, &studentA, &studentB} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}

	users := NewUserController(service.NewUserService(repository.NewUserRepository(db), db))

	// A school administrator's collection response must not disclose another tenant.
	listed := tenantContextTransactionRouter(db, users.GetUsers, schoolA.ID.String(), adminA.ID.String(), http.MethodGet, "/admin/users", "")
	if listed.Code != http.StatusOK {
		t.Fatalf("school A user list expected 200, got %d: %s", listed.Code, listed.Body.String())
	}
	var visible []models.User
	if err := json.Unmarshal(listed.Body.Bytes(), &visible); err != nil {
		t.Fatal(err)
	}
	if len(visible) != 2 {
		t.Fatalf("school A should see exactly its admin and student, got %d users", len(visible))
	}
	for _, user := range visible {
		if user.SchoolID == nil || *user.SchoolID != schoolA.ID {
			t.Fatalf("school A user list disclosed a foreign-tenant user: %+v", user)
		}
	}

	// Direct lookup by another tenant's UUID must be indistinguishable from missing.
	foreign := tenantContextTransactionRouter(db, users.GetUser, schoolA.ID.String(), adminA.ID.String(), http.MethodGet, "/admin/users/"+studentB.ID.String(), "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant user lookup expected 404, got %d: %s", foreign.Code, foreign.Body.String())
	}
	own := tenantContextTransactionRouter(db, users.GetUser, schoolA.ID.String(), adminA.ID.String(), http.MethodGet, "/admin/users/"+studentA.ID.String(), "")
	if own.Code != http.StatusOK {
		t.Fatalf("same-tenant user lookup expected 200, got %d: %s", own.Code, own.Body.String())
	}

	// Provisioning through the school-scoped service must attach the new user to A.
	created := tenantContextTransactionRouter(db, users.CreateUser, schoolA.ID.String(), adminA.ID.String(), http.MethodPost, "/admin/users",
		`{"name":"A New Student","email":"new-student-a@staging.example","password":"StagingPass123!","role":"student"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("school A user creation expected 201, got %d: %s", created.Code, created.Body.String())
	}
	var provisioned models.User
	if err := db.Where("email = ?", "new-student-a@staging.example").First(&provisioned).Error; err != nil {
		t.Fatal(err)
	}
	if provisioned.SchoolID == nil || *provisioned.SchoolID != schoolA.ID || provisioned.Role != authz.RoleStudent {
		t.Fatalf("new user was not provisioned into school A with the requested role: %+v", provisioned)
	}
}
