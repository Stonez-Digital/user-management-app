package controller

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/middleware"
)

func requireSchoolID(c *gin.Context) (uuid.UUID, bool) {
    id, ok := middleware.SchoolIDFromContext(c)
    if !ok { c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error":"school context required"}); return uuid.Nil, false }
    return id, true
}
