package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/auth"
	"github.com/onoja217/users-management-app/internal/authz"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// stagingAcceptanceDB uses the disposable PostgreSQL service supplied by CI.
// Local runs fall back to an isolated in-memory SQLite database. It never reads
// application DATABASE_URL or any production connection setting.
func stagingAcceptanceDB(t *testing.T) *gorm.DB {
	t.Helper()
	if err := auth.ConfigureSecret("staging-acceptance-test-secret-at-least-32-characters"); err != nil {
		t.Fatal(err)
	}

	var (
		db  *gorm.DB
		err error
	)
	if dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")); dsn != "" {
		// Give this test its own schema so it cannot collide with other packages
		// using the same disposable CI database.
		schema := "staging_acceptance_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		bootstrap, openErr := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if openErr != nil {
			t.Fatal(openErr)
		}
		if err := bootstrap.Exec("CREATE SCHEMA " + schema).Error; err != nil {
			t.Fatal(err)
		}
		bootstrapSQL, err := bootstrap.DB()
		if err != nil {
			t.Fatal(err)
		}
		if err := bootstrapSQL.Close(); err != nil {
			t.Fatal(err)
		}
		parsed, parseErr := url.Parse(dsn)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		db, err = gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
		if err != nil {
			dropDB, dropErr := gorm.Open(postgres.Open(dsn), &gorm.Config{})
			if dropErr == nil {
				_ = dropDB.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
				if dropSQL, e := dropDB.DB(); e == nil {
					_ = dropSQL.Close()
				}
			}
			t.Fatal(err)
		}
		sqlDB, sqlErr := db.DB()
		if sqlErr != nil {
			t.Fatal(sqlErr)
		}
		t.Cleanup(func() {
			_ = sqlDB.Close()
			dropDB, dropErr := gorm.Open(postgres.Open(dsn), &gorm.Config{})
			if dropErr == nil {
				_ = dropDB.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
				if dropSQL, e := dropDB.DB(); e == nil {
					_ = dropSQL.Close()
				}
			}
		})
	} else {
		db, err = gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	}
	if err != nil {
		t.Fatal(err)
	}

	if err := db.AutoMigrate(
		&models.School{},
		&models.User{},
		&models.RefreshToken{},
		&models.Session{},
		&models.PasswordResetToken{},
		&models.AuditLog{},
		&models.RoleChangeAudit{},
		&models.TeacherProfile{},
		&models.ParentProfile{},
	); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")) == "" {
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	return db
}

func TestIsolatedStagingAcceptanceOnboardingRecoveryAndTenantIsolation(t *testing.T) {
	db := stagingAcceptanceDB(t)
	ac := NewAuthController(db)

	// Provision two independent tenants through the public onboarding endpoint.
	const password = "StagingPass123!"
	type tenant struct {
		name, code, adminName, email string
		school                      models.School
		admin                       models.User
		accessToken                 string
	}
	tenants := []tenant{
		{name: "Staging Academy A", code: "STAGE-A", adminName: "Staging Admin A", email: "admin-a@staging.example"},
		{name: "Staging Academy B", code: "STAGE-B", adminName: "Staging Admin B", email: "admin-b@staging.example"},
	}

	for i := range tenants {
		tenant := &tenants[i]
		rec := authJSONRequest(t, ac, http.MethodPost, "/school-signup",
			`{"school_name":"`+tenant.name+`","school_code":"`+tenant.code+`","school_slug":"`+models.SchoolSlug(tenant.name, "")+`","school_type":"both","requested_plan":"trial","admin_name":"`+tenant.adminName+`","admin_email":"`+tenant.email+`","admin_password":"`+password+`"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("%s onboarding expected 201, got %d: %s", tenant.code, rec.Code, rec.Body.String())
		}
		if err := db.Where("code = ?", tenant.code).First(&tenant.school).Error; err != nil {
			t.Fatalf("%s school was not persisted: %v", tenant.code, err)
		}
		if err := db.Where("email = ?", tenant.email).First(&tenant.admin).Error; err != nil {
			t.Fatalf("%s administrator was not persisted: %v", tenant.code, err)
		}
		if tenant.admin.SchoolID == nil || *tenant.admin.SchoolID != tenant.school.ID || tenant.admin.Role != authz.RoleSchoolAdmin {
			t.Fatalf("%s administrator was not scoped to its tenant", tenant.code)
		}
		if tenant.school.Status != models.SchoolStatusPending {
			t.Fatalf("%s should require approval before login; got status %q", tenant.code, tenant.school.Status)
		}
	}

	// Pending tenants cannot authenticate. Activate only these synthetic test tenants.
	pending := authJSONRequest(t, ac, http.MethodPost, "/login",
		`{"email":"`+tenants[0].email+`","password":"`+password+`","school_code":"`+tenants[0].code+`"}`)
	if pending.Code != http.StatusUnauthorized {
		t.Fatalf("pending tenant login should be rejected, got %d: %s", pending.Code, pending.Body.String())
	}
	for i := range tenants {
		if err := db.Model(&tenants[i].school).Update("status", models.SchoolStatusActive).Error; err != nil {
			t.Fatal(err)
		}
		rec := authJSONRequest(t, ac, http.MethodPost, "/login",
			`{"email":"`+tenants[i].email+`","password":"`+password+`","school_code":"`+tenants[i].code+`"}`)
		access, _ := decodeAuthTokens(t, rec)
		tenants[i].accessToken = access
	}
	if tenants[0].school.ID == tenants[1].school.ID || tenants[0].accessToken == tenants[1].accessToken {
		t.Fatal("two-school onboarding did not produce distinct tenant identities and sessions")
	}

	// School A must not deactivate a user belonging to School B.
	target := models.User{
		Name: "School B Synthetic Student", Email: "student-b@staging.example",
		Role: authz.RoleStudent, Active: true, SchoolID: &tenants[1].school.ID,
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	denied := tenantContextRouter(
		ac.DeactivateUser,
		tenants[0].school.ID.String(),
		tenants[0].admin.ID.String(),
		http.MethodPost,
		"/admin/users/"+target.ID.String()+"/deactivate",
		"",
	)
	if denied.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant deactivation should return 404, got %d: %s", denied.Code, denied.Body.String())
	}
	var unchanged models.User
	if err := db.First(&unchanged, "id = ?", target.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !unchanged.Active || unchanged.SchoolID == nil || *unchanged.SchoolID != tenants[1].school.ID {
		t.Fatal("cross-tenant request changed the School B user")
	}

	// Exercise password recovery entirely through a test mailer; no email leaves CI.
	mailer := &passwordResetMailerStub{}
	ac.Mailer = mailer
	gin.SetMode(gin.TestMode)
	forgotRouter := gin.New()
	forgotRouter.POST("/auth/forgot-password", ac.ForgotPassword)
	forgotReq := httptest.NewRequest(http.MethodPost, "/auth/forgot-password",
		strings.NewReader(`{"email":"`+tenants[0].email+`"}`))
	forgotReq.Header.Set("Content-Type", "application/json")
	forgotRec := httptest.NewRecorder()
	forgotRouter.ServeHTTP(forgotRec, forgotReq)
	if forgotRec.Code != http.StatusOK || mailer.calls != 1 || mailer.token == "" {
		t.Fatalf("synthetic password recovery did not issue a test-mailer token: status=%d calls=%d", forgotRec.Code, mailer.calls)
	}

	resetRouter := gin.New()
	resetRouter.POST("/reset-password", ac.ResetPassword)
	resetReq := httptest.NewRequest(http.MethodPost, "/reset-password",
		strings.NewReader(`{"token":"`+mailer.token+`","new_password":"RecoveredPass456!"}`))
	resetReq.Header.Set("Content-Type", "application/json")
	resetRec := httptest.NewRecorder()
	resetRouter.ServeHTTP(resetRec, resetReq)
	if resetRec.Code != http.StatusOK {
		t.Fatalf("synthetic password reset expected 200, got %d: %s", resetRec.Code, resetRec.Body.String())
	}

	var resetToken models.PasswordResetToken
	if err := db.Where("user_id = ?", tenants[0].admin.ID).First(&resetToken).Error; err != nil {
		t.Fatal(err)
	}
	if !resetToken.Used || resetToken.TokenHash != auth.HashResetToken(mailer.token) {
		t.Fatal("password reset token was not consumed while remaining hashed")
	}
	var sessions []models.Session
	if err := db.Where("user_id = ?", tenants[0].admin.ID).Find(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	for _, session := range sessions {
		if !session.Revoked {
			t.Fatalf("password reset left session %s active", session.ID)
		}
	}

	reused := httptest.NewRequest(http.MethodPost, "/reset-password",
		strings.NewReader(`{"token":"`+mailer.token+`","new_password":"AnotherPass789!"}`))
	reused.Header.Set("Content-Type", "application/json")
	reusedRec := httptest.NewRecorder()
	resetRouter.ServeHTTP(reusedRec, reused)
	if reusedRec.Code != http.StatusUnauthorized {
		t.Fatalf("consumed reset token should be rejected, got %d: %s", reusedRec.Code, reusedRec.Body.String())
	}

	oldPassword := authJSONRequest(t, ac, http.MethodPost, "/login",
		`{"email":"`+tenants[0].email+`","password":"`+password+`","school_code":"`+tenants[0].code+`"}`)
	if oldPassword.Code != http.StatusUnauthorized {
		t.Fatalf("old password should fail after reset, got %d", oldPassword.Code)
	}
	newPassword := authJSONRequest(t, ac, http.MethodPost, "/login",
		`{"email":"`+tenants[0].email+`","password":"RecoveredPass456!","school_code":"`+tenants[0].code+`"}`)
	if newPassword.Code != http.StatusOK {
		t.Fatalf("new password should authenticate after reset, got %d: %s", newPassword.Code, newPassword.Body.String())
	}

	// The disposable database is closed by stagingAcceptanceDB cleanup.
}
