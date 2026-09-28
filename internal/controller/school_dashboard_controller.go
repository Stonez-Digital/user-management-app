package controller

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/httpx"
    "github.com/onoja217/users-management-app/internal/middleware"
    "github.com/onoja217/users-management-app/internal/service"
)

type SchoolDashboardController struct{ service *service.SchoolDashboardService }

func NewSchoolDashboardController(s *service.SchoolDashboardService) *SchoolDashboardController {
    return &SchoolDashboardController{service: s}
}

func (ctrl *SchoolDashboardController) Get(c *gin.Context) {
    schoolID, ok := middleware.SchoolIDFromContext(c)
    if !ok {
        httpx.Error(c, http.StatusForbidden, "school_context_required", "school context required")
        return
    }
    adminID, err := uuid.Parse(c.GetString("user_id"))
    if err != nil {
        httpx.Error(c, http.StatusUnauthorized, "invalid_user", "invalid authenticated user")
        return
    }
    var sessionID, termID uuid.UUID
    if raw := c.Query("academic_session_id"); raw != "" {
        sessionID, err = uuid.Parse(raw)
        if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_session_id", "invalid academic session id"); return }
    }
    if raw := c.Query("term_id"); raw != "" {
        termID, err = uuid.Parse(raw)
        if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_term_id", "invalid term id"); return }
    }
    result, err := ctrl.service.Get(schoolID, adminID, sessionID, termID)
    if err != nil {
        httpx.Error(c, http.StatusInternalServerError, "dashboard_failed", "unable to load school dashboard")
        return
    }
    if sessionID != uuid.Nil {
        result.AcademicContext.SessionID = sessionID
    }
    if termID != uuid.Nil {
        result.AcademicContext.TermID = termID
    }
    c.JSON(http.StatusOK, result)
}
