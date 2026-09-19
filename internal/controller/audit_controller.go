package controller

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type AuditController struct { DB *gorm.DB }

func NewAuditController(db *gorm.DB) *AuditController { return &AuditController{DB: db} }

func (ac *AuditController) List(c *gin.Context) {
    limit := 50
    var logs []models.AuditLog
    if err := ac.DB.Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load audit logs"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"data": logs, "limit": limit})
}
