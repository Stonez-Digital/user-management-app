package controller

import (
    "errors"
    "fmt"
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/audit"
    "github.com/onoja217/users-management-app/internal/httpx"
    "github.com/onoja217/users-management-app/internal/middleware"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/service"
    "gorm.io/gorm"
)

type SchoolController struct {
    service *service.SchoolService
    logoStorage *service.SchoolLogoStorage
}

func NewSchoolController(s *service.SchoolService, logoStorage *service.SchoolLogoStorage) *SchoolController {
    return &SchoolController{service: s, logoStorage: logoStorage}
}

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

func (ctrl *SchoolController) UploadLogo(c *gin.Context) {
    schoolID, ok := middleware.SchoolIDFromContext(c)
    if !ok {
        httpx.Error(c, http.StatusForbidden, "school_context_required", "school context required")
        return
    }
    if ctrl.logoStorage == nil {
        httpx.Error(c, http.StatusServiceUnavailable, "school_logo_storage_unavailable", "school logo storage is not configured")
        return
    }

    file, header, err := c.Request.FormFile("logo")
    if err != nil {
        httpx.Error(c, http.StatusBadRequest, "school_logo_required", "school logo file is required")
        return
    }
    defer file.Close()

    contentType := header.Header.Get("Content-Type")
    if contentType == "" {
        contentType = "application/octet-stream"
    }

    logoURL, err := ctrl.logoStorage.Upload(schoolID, header.Filename, contentType, file, header.Size)
    if err != nil {
        if strings.Contains(err.Error(), "not configured") {
            httpx.Error(c, http.StatusServiceUnavailable, "school_logo_storage_unavailable", "school logo storage is not configured")
            return
        }
        httpx.Error(c, http.StatusBadRequest, "school_logo_upload_failed", fmt.Sprintf("%v", err))
        return
    }

    if err := ctrl.service.UpdateSchool(schoolID, models.School{LogoURL: logoURL}); err != nil {
        httpx.Error(c, http.StatusInternalServerError, "school_logo_save_failed", "school logo uploaded but could not be saved")
        return
    }

    actor, _ := uuid.Parse(c.GetString("user_id"))
    _ = audit.Record(ctrl.service.DB(), c, &actor, "school.logo_upload", "school", &schoolID, nil)

    school, err := ctrl.service.GetSchool(schoolID)
    if err != nil {
        httpx.Error(c, http.StatusInternalServerError, "school_read_failed", "failed to load school")
        return
    }
    c.JSON(http.StatusOK, school)
}
