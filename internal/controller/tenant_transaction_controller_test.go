package controller

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/onoja217/users-management-app/internal/repository"
    "github.com/onoja217/users-management-app/internal/service"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestTenantControllersFailClosedWithoutRequestTransaction(t *testing.T) {
    gin.SetMode(gin.TestMode)
    db, err := gorm.Open(sqlite.Open("file:tenant-controller-fail-closed?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }

    careerPath := NewCareerPathController(service.NewCareerPathService(db))
    school := NewSchoolController(service.NewSchoolService(repository.NewSchoolRepository(db), db), nil)
    content := NewSchoolContentController(service.NewSchoolContentService(db), nil, db)
    audit := NewAuditController(db)

    routes := gin.New()
    routes.POST("/careerpath/attempts", careerPath.CreateOrResume)
    routes.GET("/admin/school", school.Get)
    routes.GET("/admin/blog/posts", content.ListBlogs)
    routes.GET("/admin/audit-logs", audit.List)

    cases := []struct {
        name string
        method string
        path string
    }{
        {"careerpath", http.MethodPost, "/careerpath/attempts"},
        {"school profile", http.MethodGet, "/admin/school"},
        {"school content", http.MethodGet, "/admin/blog/posts"},
        {"audit logs", http.MethodGet, "/admin/audit-logs"},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            request := httptest.NewRequest(tc.method, tc.path, nil)
            response := httptest.NewRecorder()
            routes.ServeHTTP(response, request)
            if response.Code != http.StatusInternalServerError {
                t.Fatalf("expected missing tenant transaction to fail with 500, got %d: %s", response.Code, response.Body.String())
            }
            if !contains(response.Body.String(), "tenant_transaction_required") {
                t.Fatalf("expected explicit tenant transaction error, got %s", response.Body.String())
            }
        })
    }
}
