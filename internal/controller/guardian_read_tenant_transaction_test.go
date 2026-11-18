package controller

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/middleware"
    "github.com/onoja217/users-management-app/internal/repository"
    "github.com/onoja217/users-management-app/internal/service"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestGuardianReadEndpointsFailClosedWithoutTenantTransaction(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file:guardian_reads_no_tx?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    ctrl := NewGuardianController(service.NewGuardianService(repository.NewGuardianRepository(db), db))
    gin.SetMode(gin.TestMode)
    routes := []struct { method, path string; handler gin.HandlerFunc }{
        {http.MethodGet, "/terms", ctrl.Terms},
        {http.MethodGet, "/children", ctrl.Children},
        {http.MethodGet, "/children/:studentId/attendance", ctrl.Attendance},
        {http.MethodGet, "/children/:studentId/report-cards/:termId", ctrl.ReportCard},
        {http.MethodGet, "/children/:studentId/timetable", ctrl.Timetable},
        {http.MethodGet, "/children/:studentId/invoices", ctrl.Invoices},
        {http.MethodGet, "/children/:studentId/payments", ctrl.Payments},
    }

    for _, route := range routes {
        t.Run(route.path, func(t *testing.T) {
            r := gin.New()
            r.Use(func(c *gin.Context) {
                c.Set(middleware.UserIDKey, uuid.NewString())
                c.Set(middleware.SchoolIDKey, uuid.NewString())
                c.Next()
            })
            switch route.path {
            case "/terms": r.GET(route.path, route.handler)
            case "/children": r.GET(route.path, route.handler)
            case "/children/:studentId/attendance": r.GET("/children/:studentId/attendance", route.handler)
            case "/children/:studentId/report-cards/:termId": r.GET("/children/:studentId/report-cards/:termId", route.handler)
            case "/children/:studentId/timetable": r.GET("/children/:studentId/timetable", route.handler)
            case "/children/:studentId/invoices": r.GET("/children/:studentId/invoices", route.handler)
            case "/children/:studentId/payments": r.GET("/children/:studentId/payments", route.handler)
            }
            path := strings.ReplaceAll(route.path, ":studentId", uuid.NewString())
            path = strings.ReplaceAll(path, ":termId", uuid.NewString())
            recorder := httptest.NewRecorder()
            r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
            if recorder.Code != http.StatusInternalServerError {
                t.Fatalf("expected missing tenant transaction to fail closed with 500, got %d: %s", recorder.Code, recorder.Body.String())
            }
            if !strings.Contains(recorder.Body.String(), "tenant_transaction_required") {
                t.Fatalf("expected tenant transaction error, got %s", recorder.Body.String())
            }
        })
    }
}
