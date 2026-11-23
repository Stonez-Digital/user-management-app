package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/auth"
	"github.com/onoja217/users-management-app/internal/authbootstrap"
)

func AuthMiddleware(store authbootstrap.Store) gin.HandlerFunc {
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
		if claims.ID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "session is no longer valid"})
			c.Abort()
			return
		}
		sessionID, err := uuid.Parse(claims.ID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "session is no longer valid"})
			c.Abort()
			return
		}
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
		if store == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "authentication temporarily unavailable"})
			c.Abort()
			return
		}

		identity, err := store.AuthenticateAccessSession(c.Request.Context(), sessionID, userID)
		if err != nil {
			if errors.Is(err, authbootstrap.ErrUnauthenticated) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "session is no longer valid"})
			} else {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "authentication temporarily unavailable"})
			}
			c.Abort()
			return
		}
		if identity.UserID != userID || strings.TrimSpace(identity.Role) == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "session is no longer valid"})
			c.Abort()
			return
		}

		c.Set(UserIDKey, identity.UserID.String())
		// Authorization always uses the live database role, never the JWT role.
		c.Set(RoleKey, identity.Role)
		if identity.SchoolID != nil && *identity.SchoolID != uuid.Nil {
			c.Set(SchoolIDKey, identity.SchoolID.String())
		} else {
			c.Set(SchoolIDKey, "")
		}
		c.Next()
	}
}
