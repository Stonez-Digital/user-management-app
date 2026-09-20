package middleware

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

const SchoolIDKey = "school_id"

func RequireSchoolContext(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID, err := uuid.Parse(c.GetString(UserIDKey))
        if err != nil { c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error":"invalid user context"}); return }
        var user models.User
        if err := db.Select("id", "school_id", "active").First(&user, "id = ?", userID).Error; err != nil { c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error":"user not found"}); return }
        if !user.Active { c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error":"account inactive"}); return }
        if user.SchoolID == nil || *user.SchoolID == uuid.Nil { c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error":"school context required"}); return }
        c.Set(SchoolIDKey, user.SchoolID.String())
        c.Next()
    }
}

func SchoolIDFromContext(c *gin.Context) (uuid.UUID, bool) {
    raw, ok := c.Get(SchoolIDKey); if !ok { return uuid.Nil, false }
    value, ok := raw.(string); if !ok { return uuid.Nil, false }
    id, err := uuid.Parse(value); return id, err == nil
}
