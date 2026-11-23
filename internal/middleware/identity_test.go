package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIdentityTransactionAllowsPlatformUserWithoutSchool(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:identity_transaction_platform?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	userID := uuid.New()
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(UserIDKey, userID.String()); c.Next() })
	router.GET("/", IdentityTransaction(db), func(c *gin.Context) {
		tx, ok := IdentityDBFromContext(c)
		if !ok || tx == nil { c.Status(http.StatusInternalServerError); return }
		c.Status(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK { t.Fatalf("platform identity transaction should not require school context: %d", rec.Code) }
}

func TestIdentityTransactionFailsClosedWithoutUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:identity_transaction_missing?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	router := gin.New()
	router.GET("/", IdentityTransaction(db), func(c *gin.Context) { c.Status(http.StatusOK) })
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized { t.Fatalf("expected missing identity to fail closed, got %d", rec.Code) }
}
