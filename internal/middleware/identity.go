package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const IdentityDBKey = "identity_db"

// IdentityTransaction opens a user-scoped transaction without requiring a school.
// Platform users therefore retain access to their own identity and session data.
func IdentityTransaction(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := uuid.Parse(c.GetString(UserIDKey))
		if err != nil || userID == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authenticated user context required"})
			return
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			if tx.Dialector.Name() == "postgres" {
				schoolID, ok := SchoolIDFromContext(c)
				schoolValue := ""
				if ok {
					schoolValue = schoolID.String()
				}
				if err := tx.Exec(
					"SELECT set_config('app.current_user_id', ?, true), set_config('app.current_school_id', ?, true)",
					userID.String(), schoolValue,
				).Error; err != nil {
					return err
				}
			}
			c.Set(IdentityDBKey, tx)
			c.Next()
			if c.IsAborted() || c.Writer.Status() >= http.StatusBadRequest {
				return errIdentityRequestRollback
			}
			return nil
		})
		c.Set(IdentityDBKey, nil)
		if err != nil && err != errIdentityRequestRollback {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "identity_transaction_failed"})
		}
	}
}

var errIdentityRequestRollback = errors.New("identity request returned an error status")

// IdentityDBFromContext returns the transaction opened by IdentityTransaction.
func IdentityDBFromContext(c *gin.Context) (*gorm.DB, bool) {
	raw, ok := c.Get(IdentityDBKey)
	if !ok || raw == nil {
		return nil, false
	}
	tx, ok := raw.(*gorm.DB)
	return tx, ok && tx != nil
}
