package audit

import (
    "encoding/json"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/middleware"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

func Record(db *gorm.DB, c *gin.Context, actorID *uuid.UUID, action, resource string, resourceID *uuid.UUID, metadata map[string]interface{}) error {
    payload := ""
    if metadata != nil {
        b, err := json.Marshal(metadata)
        if err != nil {
            return err
        }
        payload = string(b)
    }

    var schoolID *uuid.UUID
    if id, ok := middleware.SchoolIDFromContext(c); ok {
        schoolID = &id
    } else {
        lookupID := actorID
        if lookupID == nil || *lookupID == uuid.Nil {
            lookupID = resourceID
        }
        if lookupID != nil && *lookupID != uuid.Nil {
            var user models.User
            if err := db.Select("school_id").First(&user, "id = ?", *lookupID).Error; err == nil && user.SchoolID != nil {
                schoolID = user.SchoolID
            }
        }
    }

    return db.Create(&models.AuditLog{
        ID: uuid.New(),
        SchoolID: schoolID,
        ActorID: actorID,
        Action: action,
        Resource: resource,
        ResourceID: resourceID,
        IP: c.ClientIP(),
        Metadata: payload,
    }).Error
}
