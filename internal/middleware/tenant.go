package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

const (
	SchoolIDKey  = "school_id"
	TenantDBKey  = "tenant_db"
)

var errTenantRequestRollback = errors.New("tenant request returned an error status")

// RequireSchoolContext derives the tenant from the authenticated database user.
// It never trusts a school ID supplied by the client.
func RequireSchoolContext(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := uuid.Parse(c.GetString(UserIDKey))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user context"})
			return
		}
		var user models.User
		if err := db.Select("id", "school_id", "active").First(&user, "id = ?", userID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			return
		}
		if !user.Active {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "account inactive"})
			return
		}
		if user.SchoolID == nil || *user.SchoolID == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "school context required"})
			return
		}
		c.Set(SchoolIDKey, user.SchoolID.String())
		c.Next()
	}
}

func SchoolIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	raw, ok := c.Get(SchoolIDKey)
	if !ok {
		return uuid.Nil, false
	}
	value, ok := raw.(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(value)
	return id, err == nil
}

// TenantTransaction opens one transaction for the lifetime of an authenticated
// tenant request. PostgreSQL tenant/user settings are transaction-local, so they
// are cleared automatically when the transaction commits or rolls back.
//
// Handlers and services must obtain their DB handle with TenantDBFromContext;
// using the application-wide DB handle bypasses this transaction and its RLS
// context. This middleware is intentionally not installed globally until each
// tenant route has been migrated to use that request-scoped handle.
func TenantTransaction(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		schoolID, ok := SchoolIDFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "school context required"})
			return
		}
		userID, err := uuid.Parse(c.GetString(UserIDKey))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid user context"})
			return
		}

		originalWriter := c.Writer
		bufferedWriter := newTenantResponseWriter(originalWriter)
		c.Writer = bufferedWriter
		defer func() {
			c.Writer = originalWriter
			c.Set(TenantDBKey, nil)
		}()

		err = db.Transaction(func(tx *gorm.DB) error {
			if tx.Dialector.Name() == "postgres" {
				// true makes both settings LOCAL to this transaction. Never use
				// session-level SET on a pooled PostgreSQL connection.
				if err := tx.Exec(
					"SELECT set_config('app.current_school_id', ?, true), set_config('app.current_user_id', ?, true)",
					schoolID.String(),
					userID.String(),
				).Error; err != nil {
					return err
				}
			}

			c.Set(TenantDBKey, tx)
			c.Next()
			if c.IsAborted() || bufferedWriter.Status() >= http.StatusBadRequest {
				return errTenantRequestRollback
			}
			return nil
		})

		c.Writer = originalWriter
		c.Set(TenantDBKey, nil)
		switch {
		case errors.Is(err, errTenantRequestRollback):
			bufferedWriter.flushTo(originalWriter)
		case err != nil:
			originalWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
			originalWriter.WriteHeader(http.StatusInternalServerError)
			_, _ = originalWriter.Write([]byte(`{"error":"tenant_transaction_failed"}`))
		default:
			bufferedWriter.flushTo(originalWriter)
		}
	}
}

// TenantDBFromContext returns the transaction handle installed by
// TenantTransaction. The boolean is false when the route has not opted in.
func TenantDBFromContext(c *gin.Context) (*gorm.DB, bool) {
	raw, ok := c.Get(TenantDBKey)
	if !ok || raw == nil {
		return nil, false
	}
	tx, ok := raw.(*gorm.DB)
	return tx, ok && tx != nil
}
