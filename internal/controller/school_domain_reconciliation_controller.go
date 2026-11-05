package controller

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/httpx"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/gorm"
)

var reconciliationSlugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var reconciliationReservedSlugs = map[string]bool{
	"app": true, "api": true, "www": true, "mail": true, "smtp": true,
	"ftp": true, "admin": true, "autodiscover": true,
}

type SchoolDomainReconciliationController struct {
	DB *gorm.DB
	Provisioner *service.CloudflareSchoolDomainProvisioner
}

func NewSchoolDomainReconciliationController(db *gorm.DB) *SchoolDomainReconciliationController {
	return &SchoolDomainReconciliationController{DB: db, Provisioner: service.NewCloudflareSchoolDomainProvisionerFromEnv()}
}

type reconcileSchoolDomainRequest struct {
	Slug string `json:"slug" binding:"required,min=1,max=63"`
	Confirm bool `json:"confirm"`
}

// Reconcile is a preview-first, super-admin-only operation. A request without
// confirm=true is read-only. Execution attaches/verifies the new host, updates
// the school slug with an optimistic condition, then attempts safe old-host cleanup.
func (ctrl *SchoolDomainReconciliationController) Reconcile(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_school_id", "invalid school id")
		return
	}
	var req reconcileSchoolDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Validation(c, httpx.ValidationErrors(err))
		return
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if !reconciliationSlugPattern.MatchString(slug) || reconciliationReservedSlugs[slug] {
		httpx.Error(c, http.StatusBadRequest, "invalid_school_slug", "slug must be a valid, non-reserved DNS label")
		return
	}
	var school models.School
	if err := ctrl.DB.First(&school, "id = ?", id).Error; err != nil {
		httpx.Error(c, http.StatusNotFound, "school_not_found", "school not found")
		return
	}
	var collisions int64
	if err := ctrl.DB.Model(&models.School{}).Where("lower(slug) = ? AND id <> ?", slug, id).Count(&collisions).Error; err != nil {
		httpx.Error(c, http.StatusInternalServerError, "school_slug_check_failed", "unable to verify slug availability")
		return
	}
	if collisions > 0 {
		httpx.Error(c, http.StatusConflict, "school_slug_taken", "another school already uses this slug")
		return
	}
	baseDomain := service.SchoolBaseDomainFromEnv()
	newHost := slug + "." + baseDomain
	oldHost := ""
	oldSlug := strings.ToLower(strings.TrimSpace(school.Slug))
	if reconciliationSlugPattern.MatchString(oldSlug) && !reconciliationReservedSlugs[oldSlug] {
		oldHost = oldSlug + "." + baseDomain
	}
	if slug == oldSlug {
		c.JSON(http.StatusOK, gin.H{"mode":"preview", "school_id":school.ID, "current_slug":school.Slug, "requested_slug":slug, "new_hostname":newHost, "old_hostname":oldHost, "changes_required":false})
		return
	}
	if ctrl.Provisioner == nil {
		httpx.Error(c, http.StatusServiceUnavailable, "school_domain_provisioning_unavailable", "Cloudflare domain provisioning is not configured")
		return
	}
	existingNew, err := ctrl.Provisioner.LookupAttachment(c.Request.Context(), newHost)
	if err != nil {
		httpx.Error(c, http.StatusBadGateway, "school_domain_lookup_failed", "unable to verify the requested hostname in Cloudflare")
		return
	}
	if existingNew != nil && existingNew.Service != configuredWorkerName() {
		httpx.Error(c, http.StatusConflict, "school_domain_owned_elsewhere", "requested hostname is attached to a different Worker; no changes were made")
		return
	}
	var oldAttachment *service.CloudflareSchoolDomainAttachment
	if oldHost != "" && oldHost != newHost {
		oldAttachment, err = ctrl.Provisioner.LookupAttachment(c.Request.Context(), oldHost)
		if err != nil {
			httpx.Error(c, http.StatusBadGateway, "old_school_domain_lookup_failed", "unable to verify the existing hostname; no changes were made")
			return
		}
	}
	if !req.Confirm {
		c.JSON(http.StatusOK, gin.H{
			"mode":"preview", "school_id":school.ID, "school_name":school.Name,
			"current_slug":school.Slug, "requested_slug":slug,
			"new_hostname":newHost, "new_hostname_already_attached":existingNew != nil,
			"old_hostname":oldHost, "old_hostname_attached":oldAttachment != nil,
			"old_hostname_owned_by_configured_worker":oldAttachment != nil && oldAttachment.Service == configuredWorkerName(),
			"changes_required":true, "confirm_required":true,
		})
		return
	}

	attachedByThisRequest := existingNew == nil || !existingNew.Enabled
	if attachedByThisRequest {
		if err := ctrl.Provisioner.Attach(c.Request.Context(), newHost); err != nil {
			httpx.Error(c, http.StatusBadGateway, "new_school_domain_attach_failed", "Cloudflare could not attach and verify the replacement hostname; database was not changed")
			return
		}
	}
	result := ctrl.DB.Model(&models.School{}).
		Where("id = ? AND slug IS NOT DISTINCT FROM ?", id, school.Slug).
		Update("slug", slug)
	if result.Error != nil || result.RowsAffected != 1 {
		if attachedByThisRequest {
			if current, lookupErr := ctrl.Provisioner.LookupAttachment(c.Request.Context(), newHost); lookupErr == nil && current != nil && current.Service == configuredWorkerName() {
				_ = ctrl.Provisioner.DetachAttachment(c.Request.Context(), *current)
			}
		}
		httpx.Error(c, http.StatusConflict, "school_slug_update_failed", "school slug was not updated; replacement attachment compensation was attempted")
		return
	}

	cleanup := "not_required"
	if oldAttachment == nil {
		if oldHost == "" {
			cleanup = "skipped_invalid_legacy_slug"
		} else {
			cleanup = "old_hostname_not_attached"
		}
	} else if oldAttachment.Service != configuredWorkerName() {
		cleanup = "blocked_ownership_mismatch"
	} else if err := ctrl.Provisioner.DetachAttachment(c.Request.Context(), *oldAttachment); err != nil {
		cleanup = "pending_retry"
	} else {
		cleanup = "detached"
	}
	status := http.StatusOK
	if cleanup == "pending_retry" || cleanup == "blocked_ownership_mismatch" || cleanup == "skipped_invalid_legacy_slug" {
		status = http.StatusAccepted
	}
	c.JSON(status, gin.H{
		"mode":"executed", "school_id":school.ID, "previous_slug":school.Slug,
		"slug":slug, "hostname":newHost, "new_hostname_verified":true,
		"old_hostname":oldHost, "old_hostname_cleanup":cleanup,
	})
}

func configuredWorkerName() string {
	// The provisioner validates ownership against its configured Worker. The
	// value is surfaced here only for comparing lookup results, not for API calls.
	return strings.TrimSpace(getenv("CLOUDFLARE_WORKER_NAME"))
}
