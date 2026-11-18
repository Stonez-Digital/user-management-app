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
	return record(db, c, actorID, nil, false, action, resource, resourceID, metadata)
}

// RecordWithSchoolID writes an audit event with an already-resolved tenant.
// A nil schoolID is explicit (for platform-level actors) and avoids an
// additional users-table lookup from restricted authentication handlers.
func RecordWithSchoolID(db *gorm.DB, c *gin.Context, actorID, schoolID *uuid.UUID, action, resource string, resourceID *uuid.UUID, metadata map[string]interface{}) error {
	return record(db, c, actorID, schoolID, true, action, resource, resourceID, metadata)
}

func record(db *gorm.DB, c *gin.Context, actorID, explicitSchoolID *uuid.UUID, explicit bool, action, resource string, resourceID *uuid.UUID, metadata map[string]interface{}) error {
	payload := ""
	if metadata != nil {
		b, err := json.Marshal(metadata)
		if err != nil {
			return err
		}
		payload = string(b)
	}

	var schoolID *uuid.UUID
	if explicit {
		schoolID = explicitSchoolID
	} else if id, ok := middleware.SchoolIDFromContext(c); ok {
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
