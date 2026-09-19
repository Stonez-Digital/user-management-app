package audit

import (
    "encoding/json"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
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

    return db.Create(&models.AuditLog{
        ID: uuid.New(),
        ActorID: actorID,
        Action: action,
        Resource: resource,
        ResourceID: resourceID,
        IP: c.ClientIP(),
        Metadata: payload,
    }).Error
}
