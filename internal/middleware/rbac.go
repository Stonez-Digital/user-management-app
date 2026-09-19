package middleware

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/onoja217/users-management-app/internal/authz"
)

func RequirePermission(permission string) gin.HandlerFunc {
    return func(c *gin.Context) {
        role, exists := c.Get(RoleKey)
        roleName, ok := role.(string)
        if !exists || !ok || !authz.HasPermission(roleName, permission) {
            c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
            c.Abort()
            return
        }
        c.Next()
    }
}

func RequireRole(roles ...string) gin.HandlerFunc {
    allowed := make(map[string]bool, len(roles))
    for _, role := range roles {
        allowed[role] = true
    }
    return func(c *gin.Context) {
        role, exists := c.Get(RoleKey)
        roleName, ok := role.(string)
        if !exists || !ok || !allowed[roleName] {
            c.JSON(http.StatusForbidden, gin.H{"error": "role not permitted"})
            c.Abort()
            return
        }
        c.Next()
    }
}
