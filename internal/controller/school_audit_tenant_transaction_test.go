package controller

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/middleware"
)

func TestSchoolProfileHandlersFailClosedWithoutTenantTransaction(t *testing.T) {
    gin.SetMode(gin.TestMode)
    schoolID := uuid.New()
    cases := []struct {
        name string
        method string
        path string
        handler gin.HandlerFunc
        body string
    }{
        {"get", http.MethodGet, "/school", NewSchoolController(nil, nil).Get, ""},
        {"update", http.MethodPut, "/school", NewSchoolController(nil, nil).Update, `{"name":"Example School"}`},
        {"logo", http.MethodPost, "/school/logo", NewSchoolController(nil, nil).UploadLogo, ""},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            r := gin.New()
            r.Use(func(c *gin.Context) {
                c.Set(middleware.SchoolIDKey, schoolID.String())
                c.Next()
            })
            r.Handle(tc.method, tc.path, tc.handler)
            req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
            rec := httptest.NewRecorder()
            r.ServeHTTP(rec, req)
            if rec.Code != http.StatusInternalServerError {
                t.Fatalf("expected fail-closed 500, got %d: %s", rec.Code, rec.Body.String())
            }
            if !strings.Contains(rec.Body.String(), "tenant_transaction_required") {
                t.Fatalf("expected tenant_transaction_required, got %s", rec.Body.String())
            }
        })
    }
}

func TestAuditListFailsClosedWithoutTenantTransaction(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.Use(func(c *gin.Context) {
        c.Set(middleware.SchoolIDKey, uuid.New().String())
        c.Next()
    })
    r.GET("/admin/audit-logs", NewAuditController(nil).List)
    req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs", nil)
    rec := httptest.NewRecorder()
    r.ServeHTTP(rec, req)
    if rec.Code != http.StatusInternalServerError {
        t.Fatalf("expected fail-closed 500, got %d: %s", rec.Code, rec.Body.String())
    }
    if !strings.Contains(rec.Body.String(), "tenant transaction required") {
        t.Fatalf("expected tenant transaction error, got %s", rec.Body.String())
    }
}
