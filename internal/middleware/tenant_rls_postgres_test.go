package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type tenantRLSRecord struct {
	ID       uuid.UUID
	SchoolID uuid.UUID
	Value    string
}

func TestPostgresTenantTransactionEnforcesRLSAndResetsPooledContext(t *testing.T) {
	rawDSN := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if rawDSN == "" { t.Skip("TEST_DATABASE_URL is not configured") }
	admin, err := gorm.Open(postgres.Open(rawDSN), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	adminSQL, err := admin.DB()
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = adminSQL.Close() })

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	schema := "tenant_rls_" + suffix
	role := "tenant_rls_" + suffix
	password := strings.ReplaceAll(uuid.NewString(), "-", "")
	schoolA, schoolB, userID := uuid.New(), uuid.New(), uuid.New()
	table := fmt.Sprintf("\"%s\".records", schema)

	if err := admin.Exec(fmt.Sprintf("CREATE ROLE \"%s\" LOGIN PASSWORD '%s' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS", role, password)).Error; err != nil {
		t.Fatalf("create isolated NOBYPASSRLS test role: %v", err)
	}
	if err := admin.Exec(fmt.Sprintf("CREATE SCHEMA \"%s\"", schema)).Error; err != nil {
		_ = admin.Exec(fmt.Sprintf("DROP ROLE \"%s\"", role)).Error
		t.Fatalf("create isolated test schema: %v", err)
	}
	t.Cleanup(func() {
		_ = admin.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS \"%s\" CASCADE", schema)).Error
		_ = admin.Exec(fmt.Sprintf("DROP OWNED BY \"%s\"", role)).Error
		_ = admin.Exec(fmt.Sprintf("DROP ROLE IF EXISTS \"%s\"", role)).Error
	})

	if err := admin.Exec(fmt.Sprintf("CREATE TABLE %s (id uuid PRIMARY KEY, school_id uuid NOT NULL, value text NOT NULL)", table)).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec(fmt.Sprintf("INSERT INTO %s (id, school_id, value) VALUES (?, ?, ?), (?, ?, ?)", table),
		uuid.New(), schoolA, "school-a", uuid.New(), schoolB, "school-b").Error; err != nil { t.Fatal(err) }
	if err := admin.Exec(fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", table)).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec(fmt.Sprintf("ALTER TABLE %s FORCE ROW LEVEL SECURITY", table)).Error; err != nil { t.Fatal(err) }
	policy := fmt.Sprintf("CREATE POLICY tenant_isolation ON %s USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid) WITH CHECK (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid)", table)
	if err := admin.Exec(policy).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec(fmt.Sprintf("GRANT USAGE ON SCHEMA \"%s\" TO \"%s\"", schema, role)).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec(fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON %s TO \"%s\"", table, role)).Error; err != nil { t.Fatal(err) }

	parsed, err := url.Parse(rawDSN)
	if err != nil || parsed.Scheme == "" { t.Fatalf("TEST_DATABASE_URL must be a PostgreSQL URL: %v", err) }
	parsed.User = url.UserPassword(role, password)
	restricted, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	if err != nil { t.Fatalf("connect using isolated NOBYPASSRLS role: %v", err) }
	restrictedSQL, err := restricted.DB()
	if err != nil { t.Fatal(err) }
	restrictedSQL.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = restrictedSQL.Close() })

	var roleFlags struct { Superuser bool; BypassRLS bool }
	if err := restricted.Raw("SELECT rolsuper AS superuser, rolbypassrls AS bypass_rls FROM pg_roles WHERE rolname = current_user").Scan(&roleFlags).Error; err != nil { t.Fatal(err) }
	if roleFlags.Superuser || roleFlags.BypassRLS { t.Fatalf("test role must be restricted; superuser=%t bypassrls=%t", roleFlags.Superuser, roleFlags.BypassRLS) }

	runRequest := func(school uuid.UUID) (*httptest.ResponseRecorder, map[string]interface{}) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(UserIDKey, userID.String())
			c.Set(SchoolIDKey, school.String())
			c.Next()
		})
		r.Use(TenantTransaction(restricted))
		r.GET("/", func(c *gin.Context) {
			tx, ok := TenantDBFromContext(c)
			if !ok { c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant transaction missing"}); return }
			var rows []tenantRLSRecord
			if err := tx.Table(table).Find(&rows).Error; err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": "tenant select failed"}); return }
			response := map[string]interface{}{"visible": len(rows)}
			if school == schoolA {
				updated := tx.Table(table).Where("school_id = ?", schoolB).Update("value", "cross-school-modified")
				response["cross_update"] = updated.RowsAffected
				deleted := tx.Table(table).Where("school_id = ?", schoolB).Delete(&tenantRLSRecord{})
				response["cross_delete"] = deleted.RowsAffected
				crossInsertErr := tx.Transaction(func(savepoint *gorm.DB) error {
					return savepoint.Table(table).Create(map[string]interface{}{"id": uuid.New(), "school_id": schoolB, "value": "cross-school-insert"}).Error
				})
				response["cross_insert_denied"] = crossInsertErr != nil
				sameInsert := tx.Table(table).Create(map[string]interface{}{"id": uuid.New(), "school_id": schoolA, "value": "same-school-insert"})
				response["same_insert"] = sameInsert.Error == nil
			}
			var userContext string
			if err := tx.Raw("SELECT current_setting('app.current_user_id', true)").Scan(&userContext).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "user context unavailable"})
				return
			}
			response["user_context"] = userContext
			c.JSON(http.StatusOK, response)
		})
		recorder := httptest.NewRecorder()
		r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
		response := map[string]interface{}{}
		_ = json.Unmarshal(recorder.Body.Bytes(), &response)
		return recorder, response
	}

	recorderA, responseA := runRequest(schoolA)
	if recorderA.Code != http.StatusOK { t.Fatalf("school A request failed: %d %s", recorderA.Code, recorderA.Body.String()) }
	if responseA["visible"] != float64(1) || responseA["cross_update"] != float64(0) || responseA["cross_delete"] != float64(0) || responseA["cross_insert_denied"] != true || responseA["same_insert"] != true {
		t.Fatalf("school A RLS checks failed: %+v", responseA)
	}
	if responseA["user_context"] != userID.String() { t.Fatalf("expected transaction-local user context %s, got %v", userID, responseA["user_context"]) }

	recorderB, responseB := runRequest(schoolB)
	if recorderB.Code != http.StatusOK || responseB["visible"] != float64(1) {
		t.Fatalf("school B must see only its own row after pooled connection reuse; status=%d response=%+v body=%s", recorderB.Code, responseB, recorderB.Body.String())
	}

	var leaked int64
	if err := restricted.Table(table).Count(&leaked).Error; err != nil { t.Fatal(err) }
	if leaked != 0 { t.Fatalf("expected transaction-local tenant context to be cleared after commit, saw %d rows without context", leaked) }
}
