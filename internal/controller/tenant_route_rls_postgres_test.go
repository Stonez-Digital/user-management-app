package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This test exercises the real school-content HTTP handlers and their
// request-scoped transaction against PostgreSQL RLS using a restricted role.
// It deliberately uses an isolated schema so it cannot touch application data.
func TestSchoolContentRoutesEnforceTwoSchoolRLS(t *testing.T) {
	rawDSN := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if rawDSN == "" {
		if strings.EqualFold(os.Getenv("CI"), "true") {
			t.Fatal("TEST_DATABASE_URL is required in CI; refusing to skip the school-content route RLS test")
		}
		t.Skip("TEST_DATABASE_URL is not configured outside CI")
	}

	admin, err := gorm.Open(postgres.Open(rawDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	adminSQL.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = adminSQL.Close() })

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	schema := "school_route_rls_" + suffix
	role := "school_route_rls_" + suffix
	password := strings.ReplaceAll(uuid.NewString(), "-", "")
	schoolA, schoolB, userID := uuid.New(), uuid.New(), uuid.New()
	table := fmt.Sprintf("\"%s\".blog_posts", schema)

	if err := admin.Exec(fmt.Sprintf("CREATE ROLE \"%s\" LOGIN PASSWORD '%s' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS", role, password)).Error; err != nil {
		t.Fatalf("create restricted role: %v", err)
	}
	if err := admin.Exec(fmt.Sprintf("CREATE SCHEMA \"%s\"", schema)).Error; err != nil {
		_ = admin.Exec(fmt.Sprintf("DROP ROLE \"%s\"", role)).Error
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS \"%s\" CASCADE", schema)).Error
		_ = admin.Exec(fmt.Sprintf("DROP OWNED BY \"%s\"", role)).Error
		_ = admin.Exec(fmt.Sprintf("DROP ROLE IF EXISTS \"%s\"", role)).Error
	})

	// Build the table from the production GORM model rather than a hand-written
	// approximation, so selected columns, nullability, defaults and indexes track
	// the model used by SchoolContentService.
	if err := admin.Exec(fmt.Sprintf("SET search_path TO \"%s\", public", schema)).Error; err != nil {
		t.Fatalf("set isolated schema search path: %v", err)
	}
	if err := admin.AutoMigrate(&models.BlogPost{}); err != nil {
		t.Fatalf("auto-migrate production BlogPost model: %v", err)
	}
	if err := admin.Create(&models.BlogPost{ID: uuid.New(), SchoolID: schoolA, AuthorID: userID, Title: "School A post", Slug: "school-a-post", Content: "A content", Status: models.ContentPublished}).Error; err != nil {
		t.Fatalf("seed School A blog post: %v", err)
	}
	if err := admin.Create(&models.BlogPost{ID: uuid.New(), SchoolID: schoolB, AuthorID: uuid.New(), Title: "School B post", Slug: "school-b-post", Content: "B content", Status: models.ContentPublished}).Error; err != nil {
		t.Fatalf("seed School B blog post: %v", err)
	}
	if err := admin.Exec(fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", table)).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec(fmt.Sprintf("ALTER TABLE %s FORCE ROW LEVEL SECURITY", table)).Error; err != nil {
		t.Fatal(err)
	}
	policy := fmt.Sprintf("CREATE POLICY school_isolation ON %s USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid) WITH CHECK (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid)", table)
	if err := admin.Exec(policy).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec(fmt.Sprintf("GRANT USAGE ON SCHEMA \"%s\" TO \"%s\"", schema, role)).Error; err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec(fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON %s TO \"%s\"", table, role)).Error; err != nil {
		t.Fatal(err)
	}

	parsed, err := url.Parse(rawDSN)
	if err != nil || parsed.Scheme == "" {
		t.Fatalf("TEST_DATABASE_URL must be a PostgreSQL URL: %v", err)
	}
	parsed.User = url.UserPassword(role, password)
	restricted, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect with restricted database role: %v", err)
	}
	restrictedSQL, err := restricted.DB()
	if err != nil {
		t.Fatal(err)
	}
	restrictedSQL.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = restrictedSQL.Close() })
	if err := restricted.Exec(fmt.Sprintf("SET search_path TO \"%s\", public", schema)).Error; err != nil {
		t.Fatalf("set isolated schema search path: %v", err)
	}

	var flags struct {
		Superuser bool
		BypassRLS bool
	}
	if err := restricted.Raw("SELECT rolsuper AS superuser, rolbypassrls AS bypass_rls FROM pg_roles WHERE rolname = current_user").Scan(&flags).Error; err != nil {
		t.Fatal(err)
	}
	if flags.Superuser || flags.BypassRLS {
		t.Fatalf("route test role must be restricted; superuser=%t bypassrls=%t", flags.Superuser, flags.BypassRLS)
	}

	gin.SetMode(gin.TestMode)
	controller := NewSchoolContentController(service.NewSchoolContentService(restricted), nil, restricted)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.UserIDKey, userID.String())
		c.Set(middleware.SchoolIDKey, schoolA.String())
		c.Next()
	})
	router.Use(middleware.TenantTransaction(restricted))
	router.GET("/blogs", controller.ListBlogs)
	router.PUT("/blogs/:id", controller.UpdateBlog)

	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/blogs", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("school A blog list failed: status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	var listed struct {
		Posts []models.BlogPost `json:"posts"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode blog list response: %v", err)
	}
	if len(listed.Posts) != 1 || listed.Posts[0].SchoolID != schoolA || listed.Posts[0].Title != "School A post" {
		t.Fatalf("school A must see only its own post, got %+v", listed.Posts)
	}

	var schoolBPost models.BlogPost
	if err := admin.Table(table).Where("school_id = ?", schoolB).First(&schoolBPost).Error; err != nil {
		t.Fatal(err)
	}
	updateBody := []byte(`{"title":"compromised by school A"}`)
	crossSchoolUpdate := httptest.NewRecorder()
	router.ServeHTTP(crossSchoolUpdate, httptest.NewRequest(http.MethodPut, "/blogs/"+schoolBPost.ID.String(), bytes.NewReader(updateBody)))
	if crossSchoolUpdate.Code != http.StatusNotFound {
		t.Fatalf("school A must not update school B's post; expected 404, got %d: %s", crossSchoolUpdate.Code, crossSchoolUpdate.Body.String())
	}
	var unchanged models.BlogPost
	if err := admin.Table(table).Where("id = ?", schoolBPost.ID).First(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Title != "School B post" {
		t.Fatalf("school B's post was modified by school A: %q", unchanged.Title)
	}

	schoolAPostID := listed.Posts[0].ID
	sameSchoolUpdate := httptest.NewRecorder()
	router.ServeHTTP(sameSchoolUpdate, httptest.NewRequest(http.MethodPut, "/blogs/"+schoolAPostID.String(), bytes.NewReader([]byte(`{"title":"School A updated"}`))))
	if sameSchoolUpdate.Code != http.StatusOK {
		t.Fatalf("school A should be able to update its own post; status=%d body=%s", sameSchoolUpdate.Code, sameSchoolUpdate.Body.String())
	}
	var updated models.BlogPost
	if err := admin.Table(table).Where("id = ?", schoolAPostID).First(&updated).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Title != "School A updated" {
		t.Fatalf("school A update was not persisted: %q", updated.Title)
	}
}
