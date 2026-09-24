package controller

import (
    "errors"
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/audit"
    "github.com/onoja217/users-management-app/internal/httpx"
    "github.com/onoja217/users-management-app/internal/middleware"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/service"
    "gorm.io/gorm"
)

type SchoolController struct{ service *service.SchoolService }

func NewSchoolController(s *service.SchoolService) *SchoolController { return &SchoolController{service: s} }

type UpdateSchoolRequest struct {
    Name string `json:"name" binding:"required,min=2,max=160"`
    LogoURL string `json:"logo_url" binding:"omitempty,max=1000"`
}

func (ctrl *SchoolController) Get(c *gin.Context) {
    schoolID, ok := middleware.SchoolIDFromContext(c)
    if !ok { httpx.Error(c, http.StatusForbidden, "school_context_required", "school context required"); return }

    school, err := ctrl.service.GetSchool(schoolID)
    if errors.Is(err, service.ErrSchoolNotFound) { httpx.Error(c, http.StatusNotFound, "school_not_found", "school not found"); return }
    if err != nil { httpx.Error(c, http.StatusInternalServerError, "school_read_failed", "failed to load school"); return }

    c.JSON(http.StatusOK, school)
}

func (ctrl *SchoolController) Update(c *gin.Context) {
    schoolID, ok := middleware.SchoolIDFromContext(c)
    if !ok { httpx.Error(c, http.StatusForbidden, "school_context_required", "school context required"); return }

    var req UpdateSchoolRequest
    if err := c.ShouldBindJSON(&req); err != nil { httpx.Validation(c, httpx.ValidationErrors(err)); return }

    if err := ctrl.service.UpdateSchool(schoolID, models.School{Name: req.Name, LogoURL: req.LogoURL}); err != nil {
        switch {
        case errors.Is(err, service.ErrSchoolNotFound), errors.Is(err, gorm.ErrRecordNotFound):
            httpx.Error(c, http.StatusNotFound, "school_not_found", "school not found")
        case errors.Is(err, service.ErrInvalidSchoolName):
            httpx.Error(c, http.StatusBadRequest, "school_update_invalid", "invalid school name")
        default:
            httpx.Error(c, http.StatusInternalServerError, "school_update_failed", "failed to update school")
        }
        return
    }

    actor, _ := uuid.Parse(c.GetString("user_id"))
    _ = audit.Record(ctrl.service.DB(), c, &actor, "school.update", "school", &schoolID, nil)

    school, err := ctrl.service.GetSchool(schoolID)
    if err != nil { httpx.Error(c, http.StatusInternalServerError, "school_read_failed", "failed to load school"); return }
    c.JSON(http.StatusOK, school)
}
