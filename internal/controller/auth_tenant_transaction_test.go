package controller

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
)

func TestAuthControllerRoleAndActivationRequireTenantTransaction(t *testing.T) {
    gin.SetMode(gin.TestMode)
    for _, path := range []string{"/admin/users/00000000-0000-0000-0000-000000000001/role", "/admin/users/00000000-0000-0000-0000-000000000001/activate"} {
        recorder := httptest.NewRecorder()
        ctx, _ := gin.CreateTestContext(recorder)
        ctx.Request = httptest.NewRequest(http.MethodPost, path, nil)
        ctrl := NewAuthController(nil)
        if _, ok := ctrl.requestTenantDB(ctx); ok {
            t.Fatalf("auth controller accepted %s without tenant transaction", path)
        }
        if recorder.Code != http.StatusInternalServerError {
            t.Fatalf("expected 500 for %s without tenant transaction, got %d", path, recorder.Code)
        }
    }
}
