package controller

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/onoja217/users-management-app/internal/middleware"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type AuditController struct { DB *gorm.DB }

func NewAuditController(db *gorm.DB) *AuditController { return &AuditController{DB: db} }

func (ac *AuditController) List(c *gin.Context) {
    schoolID, ok := middleware.SchoolIDFromContext(c)
    if !ok {
        c.JSON(http.StatusForbidden, gin.H{"error": "school context required"})
        return
    }
    limit := 50
    var logs []models.AuditLog
    if err := ac.DB.Where("school_id = ?", schoolID).Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit logs"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"data": logs, "limit": limit})
}
