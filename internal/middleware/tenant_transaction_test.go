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

type tenantTransactionRecord struct {
	ID       uint
	SchoolID string
	Value    string
}

func tenantTransactionSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:tenant_tx_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	if err := db.AutoMigrate(&tenantTransactionRecord{}); err != nil { t.Fatal(err) }
	return db
}

func tenantTransactionRequest(db *gorm.DB, schoolID, userID uuid.UUID, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, userID.String())
		c.Set(SchoolIDKey, schoolID.String())
		c.Next()
	})
	r.Use(TenantTransaction(db))
	r.GET("/", handler)
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	return recorder
}

func TestTenantTransactionCommitsWithRequestScopedHandle(t *testing.T) {
	db := tenantTransactionSQLite(t)
	schoolID, userID := uuid.New(), uuid.New()
	recorder := tenantTransactionRequest(db, schoolID, userID, func(c *gin.Context) {
		tx, ok := TenantDBFromContext(c)
		if !ok || tx == db {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "request transaction missing"})
			return
		}
		if err := tx.Create(&tenantTransactionRecord{SchoolID: schoolID.String(), Value: "committed"}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	if recorder.Code != http.StatusOK { t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String()) }
	var count int64
	if err := db.Model(&tenantTransactionRecord{}).Count(&count).Error; err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("expected successful request transaction to commit one row, got %d", count) }
}

func TestTenantTransactionRollsBackErrorResponses(t *testing.T) {
	db := tenantTransactionSQLite(t)
	schoolID, userID := uuid.New(), uuid.New()
	recorder := tenantTransactionRequest(db, schoolID, userID, func(c *gin.Context) {
		tx, ok := TenantDBFromContext(c)
		if !ok { c.JSON(http.StatusInternalServerError, gin.H{"error": "request transaction missing"}); return }
		if err := tx.Create(&tenantTransactionRecord{SchoolID: schoolID.String(), Value: "must rollback"}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "write failed"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "rejected"})
	})
	if recorder.Code != http.StatusConflict { t.Fatalf("expected original 409 response, got %d: %s", recorder.Code, recorder.Body.String()) }
	var count int64
	if err := db.Model(&tenantTransactionRecord{}).Count(&count).Error; err != nil { t.Fatal(err) }
	if count != 0 { t.Fatalf("expected error response to roll back transaction, found %d rows", count) }
}

func TestTenantTransactionRequiresAuthenticatedTenantContext(t *testing.T) {
	db := tenantTransactionSQLite(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(TenantTransaction(db))
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusForbidden { t.Fatalf("expected missing tenant context to be denied, got %d", recorder.Code) }
}
