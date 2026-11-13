package controller

import (
    "net/http"
    "github.com/onoja217/users-management-app/internal/middleware"
    "github.com/gin-gonic/gin"
    "github.com/onoja217/users-management-app/internal/httpx"
    "github.com/onoja217/users-management-app/internal/service"
)

type AcademicReadinessController struct { service *service.AcademicReadinessService }

func NewAcademicReadinessController(s *service.AcademicReadinessService) *AcademicReadinessController {
    return &AcademicReadinessController{service: s}
}

func (ctrl *AcademicReadinessController) Get(c *gin.Context) {
    tx, ok := middleware.TenantDBFromContext(c)
    if !ok { httpx.Error(c, http.StatusInternalServerError, "tenant_transaction_required", "tenant transaction is required for this operation"); return }
    svc := ctrl.service.WithDB(tx)
    schoolID, ok := requireSchoolID(c)
    if !ok { return }
    readiness, err := svc.Get(schoolID)
    if err != nil {
        httpx.Error(c, http.StatusInternalServerError, "academic_readiness_failed", "failed to load academic readiness")
        return
    }
    c.JSON(http.StatusOK, readiness)
}
