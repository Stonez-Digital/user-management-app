package middleware

import (
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "github.com/onoja217/users-management-app/internal/auth"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

func AuthMiddleware(db *gorm.DB) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
            c.Abort()
            return
        }
        if !strings.HasPrefix(authHeader, "Bearer ") {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token format"})
            c.Abort()
            return
        }
        tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
        claims, err := auth.ParseToken(tokenString)
        if err != nil || claims.Role == "refresh" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
            c.Abort()
            return
        }

        var user models.User
        if err := db.Select("id, active, role").First(&user, "id = ?", claims.UserID).Error; err != nil || !user.Active {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "account inactive or unavailable"})
            c.Abort()
            return
        }

        c.Set(UserIDKey, claims.UserID)
        // Always use the current database role. This makes role changes take effect
        // immediately instead of waiting for an old access token to expire.
        c.Set(RoleKey, user.Role)
        c.Next()
    }
}
