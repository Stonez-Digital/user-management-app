package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/audit"
	"github.com/onoja217/users-management-app/internal/httpx"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
)

type BulkImportController struct { service *service.BulkImportService }

func NewBulkImportController(s *service.BulkImportService) *BulkImportController {
	return &BulkImportController{service: s}
}

func (ctrl *BulkImportController) requestService(c *gin.Context) (*service.BulkImportService, bool) {
	tx, ok := middleware.TenantDBFromContext(c)
	if !ok || tx == nil {
		httpx.Error(c, http.StatusInternalServerError, "tenant_transaction_required", "tenant transaction is required for this operation")
		return nil, false
	}
	return ctrl.service.WithDB(tx), true
}

func (ctrl *BulkImportController) Create(c *gin.Context) {
	schoolID, ok := middleware.SchoolIDFromContext(c)
	if !ok { httpx.Error(c, http.StatusForbidden, "school_context_required", "school context required"); return }
	svc, ready := ctrl.requestService(c); if !ready { return }
	actor, err := uuid.Parse(c.GetString("user_id"))
	if err != nil { httpx.Error(c, http.StatusUnauthorized, "invalid_user", "invalid authenticated user"); return }
	kind := strings.ToLower(strings.TrimSpace(c.PostForm("kind")))
	file, err := c.FormFile("file")
	if err != nil { httpx.Error(c, http.StatusBadRequest, "file_required", "CSV or XLSX file is required"); return }
	rows, err := service.ParseUpload(file)
	if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_import_file", err.Error()); return }
	sendCredentials := strings.EqualFold(strings.TrimSpace(c.PostForm("send_credentials")), "true")
	out, err := svc.Create(schoolID, actor, kind, file.Filename, sendCredentials, rows)
	if err != nil { httpx.Error(c, http.StatusBadRequest, "import_validation_failed", err.Error()); return }
	_ = audit.Record(svc.DB(), c, &actor, "school.bulk_import.preview", "bulk_import", &out.Job.ID, map[string]interface{}{"kind": kind, "total": out.Job.Total, "valid": out.Job.Valid})
	c.JSON(http.StatusCreated, out)
}

func (ctrl *BulkImportController) Start(c *gin.Context) {
	schoolID, ok := middleware.SchoolIDFromContext(c)
	if !ok { httpx.Error(c, http.StatusForbidden, "school_context_required", "school context required"); return }
	svc, ready := ctrl.requestService(c); if !ready { return }
	actor, err := uuid.Parse(c.GetString("user_id"))
	if err != nil { httpx.Error(c, http.StatusUnauthorized, "invalid_user", "invalid authenticated user"); return }
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_import_id", "invalid import id"); return }
	if err = svc.Start(schoolID, actor, id); err != nil {
		if strings.Contains(err.Error(), "not pending") { httpx.Error(c, http.StatusConflict, "import_not_pending", err.Error()); return }
		httpx.Error(c, http.StatusNotFound, "import_not_found", "import job not found"); return
	}
	_ = audit.Record(svc.DB(), c, &actor, "school.bulk_import.start", "bulk_import", &id, nil)
	c.JSON(http.StatusAccepted, gin.H{"id": id, "status": models.BulkImportQueued})
}

func (ctrl *BulkImportController) List(c *gin.Context) {
	schoolID, ok := middleware.SchoolIDFromContext(c)
	if !ok { httpx.Error(c, http.StatusForbidden, "school_context_required", "school context required"); return }
	svc, ready := ctrl.requestService(c); if !ready { return }
	jobs, err := svc.List(schoolID)
	if err != nil { httpx.Error(c, http.StatusInternalServerError, "imports_failed", "unable to list imports"); return }
	c.JSON(http.StatusOK, gin.H{"imports": jobs})
}

func (ctrl *BulkImportController) Get(c *gin.Context) {
	schoolID, ok := middleware.SchoolIDFromContext(c)
	if !ok { httpx.Error(c, http.StatusForbidden, "school_context_required", "school context required"); return }
	svc, ready := ctrl.requestService(c); if !ready { return }
	id, err := uuid.Parse(c.Param("id"))
	if err != nil { httpx.Error(c, http.StatusBadRequest, "invalid_import_id", "invalid import id"); return }
	job, err := svc.Get(schoolID, id)
	if err != nil { httpx.Error(c, http.StatusNotFound, "import_not_found", "import job not found"); return }
	c.JSON(http.StatusOK, job)
}
