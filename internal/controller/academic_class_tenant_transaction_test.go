package controller

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/onoja217/users-management-app/internal/repository"
    "github.com/onoja217/users-management-app/internal/service"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestAcademicAndClassEndpointsFailClosedWithoutTenantTransaction(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file:academic_class_no_tx?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }

    academic := NewAcademicController(service.NewAcademicService(
        repository.NewAcademicSessionRepository(db),
        repository.NewTermRepository(db),
        db,
    ))
    classes := NewClassController(service.NewClassService(
        repository.NewClassRepository(db),
        repository.NewSectionRepository(db),
        db,
    ))

    gin.SetMode(gin.TestMode)
    routes := []struct {
        method string
        path string
        handler gin.HandlerFunc
    }{
        {http.MethodGet, "/academic-context", academic.GetContext},
        {http.MethodGet, "/admin/academic-sessions", academic.ListSessions},
        {http.MethodGet, "/admin/academic-sessions/:id", academic.GetSession},
        {http.MethodPost, "/admin/academic-sessions", academic.CreateSession},
        {http.MethodPut, "/admin/academic-sessions/:id", academic.UpdateSession},
        {http.MethodDelete, "/admin/academic-sessions/:id", academic.DeleteSession},
        {http.MethodGet, "/admin/academic-sessions/:id/terms", academic.ListTerms},
        {http.MethodPost, "/admin/academic-sessions/:id/terms", academic.CreateTerm},
        {http.MethodGet, "/admin/terms/:id", academic.GetTerm},
        {http.MethodPut, "/admin/terms/:id", academic.UpdateTerm},
        {http.MethodDelete, "/admin/terms/:id", academic.DeleteTerm},
        {http.MethodGet, "/admin/classes", classes.ListClasses},
        {http.MethodGet, "/admin/classes/:id", classes.GetClass},
        {http.MethodPost, "/admin/classes", classes.CreateClass},
        {http.MethodPut, "/admin/classes/:id", classes.UpdateClass},
        {http.MethodDelete, "/admin/classes/:id", classes.DeleteClass},
        {http.MethodGet, "/admin/classes/:id/sections", classes.ListSections},
        {http.MethodPost, "/admin/classes/:id/sections", classes.CreateSection},
        {http.MethodGet, "/admin/sections/:id", classes.GetSection},
        {http.MethodPut, "/admin/sections/:id", classes.UpdateSection},
        {http.MethodDelete, "/admin/sections/:id", classes.DeleteSection},
    }

    for _, route := range routes {
        t.Run(route.method+" "+route.path, func(t *testing.T) {
            r := gin.New()
            r.Handle(route.method, route.path, route.handler)
            path := strings.ReplaceAll(route.path, ":id", "11111111-1111-4111-8111-111111111111")
            recorder := httptest.NewRecorder()
            r.ServeHTTP(recorder, httptest.NewRequest(route.method, path, nil))
            if recorder.Code != http.StatusInternalServerError {
                t.Fatalf("expected missing tenant transaction to fail closed with 500, got %d: %s", recorder.Code, recorder.Body.String())
            }
            if !strings.Contains(recorder.Body.String(), "tenant_transaction_required") {
                t.Fatalf("expected tenant transaction error, got %s", recorder.Body.String())
            }
        })
    }
}
