package controller

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/onoja217/users-management-app/internal/service"
)

func TestFinanceControllerRequiresTenantTransaction(t *testing.T) {
    gin.SetMode(gin.TestMode)
    recorder := httptest.NewRecorder()
    ctx, _ := gin.CreateTestContext(recorder)
    ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/fees", nil)
    ctrl := NewFinanceController(service.NewFinanceService(nil, nil, nil, nil))

    if _, ok := ctrl.requestService(ctx); ok {
        t.Fatal("finance controller accepted a request without tenant transaction")
    }
    if recorder.Code != http.StatusInternalServerError {
        t.Fatalf("expected 500 when tenant transaction is missing, got %d", recorder.Code)
    }
}
