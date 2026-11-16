package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		if claims.ID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "session is no longer valid"})
			c.Abort()
			return
		}
		userID, err := uuid.Parse(claims.UserID)
		if err != nil || userID == uuid.Nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user identity"})
			c.Abort()
			return
		}
		sessionID, err := uuid.Parse(claims.ID)
		if err != nil || sessionID == uuid.Nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "session is no longer valid"})
			c.Abort()
			return
		}

		var user models.User
		err = db.Transaction(func(tx *gorm.DB) error {
			if tx.Dialector.Name() == "postgres" {
				// The JWT signature has been verified. Set only the signed user
				// identity before reading self-owned session/user rows.
				if err := tx.Exec(
					"SELECT set_config('app.current_user_id', ?, true)",
					userID.String(),
				).Error; err != nil {
					return err
				}
			}
			var session models.Session
			if err := tx.Where(
				"id = ? AND user_id = ? AND revoked = false AND expires_at > ?",
				sessionID, userID, time.Now(),
			).First(&session).Error; err != nil {
				return err
			}
			if err := tx.Select("id, active, role, school_id").First(&user, "id = ?", userID).Error; err != nil {
				return err
			}
			if !user.Active {
				return gorm.ErrRecordNotFound
			}
			if tx.Dialector.Name() == "postgres" && user.SchoolID != nil && *user.SchoolID != uuid.Nil {
				if err := tx.Exec(
					"SELECT set_config('app.current_school_id', ?, true)",
					user.SchoolID.String(),
				).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "session or account is no longer valid"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "authentication_context_unavailable"})
			}
			c.Abort()
			return
		}

		c.Set(UserIDKey, userID.String())
		// Always use the current database role so role changes take effect
		// immediately instead of waiting for an old access token to expire.
		c.Set(RoleKey, user.Role)
		if user.SchoolID != nil && *user.SchoolID != uuid.Nil {
			c.Set(SchoolIDKey, user.SchoolID.String())
		}
		c.Next()
	}
}
