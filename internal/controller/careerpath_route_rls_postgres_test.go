package controller

import (
	"bytes"
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
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// This exercises the real CareerPath handlers and service against an isolated
// PostgreSQL schema under a NOSUPERUSER/NOBYPASSRLS role.
func TestCareerPathRoutesEnforceTwoSchoolRLS(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		if strings.EqualFold(os.Getenv("CI"), "true") { t.Fatal("TEST_DATABASE_URL is required in CI; refusing to skip CareerPath route RLS test") }
		t.Skip("TEST_DATABASE_URL is not configured outside CI")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil { t.Fatalf("open PostgreSQL test database: %v", err) }
	adminSQL, err := admin.DB(); if err != nil { t.Fatal(err) }; adminSQL.SetMaxOpenConns(1); t.Cleanup(func(){ _ = adminSQL.Close() })

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	schema, role := "careerpath_rls_"+suffix, "careerpath_rls_"+suffix
	password := strings.ReplaceAll(uuid.NewString(), "-", "")
	schoolA, schoolB, userA, userB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	studentA, studentB, attemptB := uuid.New(), uuid.New(), uuid.New()
	if err := admin.Exec(fmt.Sprintf("CREATE ROLE \"%s\" LOGIN PASSWORD '%s' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS", role, password)).Error; err != nil { t.Fatalf("create restricted role: %v", err) }
	if err := admin.Exec(fmt.Sprintf("CREATE SCHEMA \"%s\"", schema)).Error; err != nil { _ = admin.Exec(fmt.Sprintf("DROP ROLE \"%s\"", role)).Error; t.Fatalf("create schema: %v", err) }
	t.Cleanup(func(){ _ = admin.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS \"%s\" CASCADE",schema)).Error; _ = admin.Exec(fmt.Sprintf("DROP OWNED BY \"%s\"",role)).Error; _ = admin.Exec(fmt.Sprintf("DROP ROLE IF EXISTS \"%s\"",role)).Error })
	if err := admin.Exec(fmt.Sprintf("SET search_path TO \"%s\", public",schema)).Error; err != nil { t.Fatal(err) }
	if err := admin.Exec(`CREATE TABLE students (id uuid PRIMARY KEY, school_id uuid NOT NULL, user_id uuid NOT NULL)`).Error; err != nil { t.Fatalf("create students fixture: %v",err) }
	if err := admin.Exec(`CREATE TABLE career_path_attempts (
		id uuid PRIMARY KEY, school_id uuid NOT NULL, student_id uuid NOT NULL, user_id uuid NOT NULL,
		version varchar(32) NOT NULL, status varchar(20) NOT NULL,
		responses text NOT NULL DEFAULT '{}'::text, results text NOT NULL DEFAULT '[]'::text,
		submitted_at timestamptz NULL, created_at timestamptz NULL, updated_at timestamptz NULL
	)`).Error; err != nil { t.Fatalf("create CareerPath fixture: %v",err) }
	if err := admin.Exec(`CREATE UNIQUE INDEX uq_careerpath_one_draft ON career_path_attempts (school_id, student_id, version) WHERE status = 'in_progress'`).Error; err != nil { t.Fatal(err) }
	for _, stmt := range []string{
		"ALTER TABLE career_path_attempts ENABLE ROW LEVEL SECURITY",
		"ALTER TABLE career_path_attempts FORCE ROW LEVEL SECURITY",
		"CREATE POLICY careerpath_school_isolation ON career_path_attempts USING (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid) WITH CHECK (school_id = nullif(current_setting('app.current_school_id', true), '')::uuid)",
		fmt.Sprintf("GRANT USAGE ON SCHEMA \"%s\" TO \"%s\"",schema,role),
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON students, career_path_attempts TO \"%s\"",role),
	} { if err := admin.Exec(stmt).Error; err != nil { t.Fatalf("configure isolated RLS: %v; SQL=%s",err,stmt) } }
	if err := admin.Exec("INSERT INTO students (id,school_id,user_id) VALUES (?,?,?),(?,?,?)",studentA,schoolA,userA,studentB,schoolB,userB).Error; err != nil { t.Fatalf("seed students: %v",err) }
	if err := admin.Exec("INSERT INTO career_path_attempts (id,school_id,student_id,user_id,version,status,responses,results) VALUES (?,?,?,?,?,?,?,?)",attemptB,schoolB,studentB,userB,service.CareerPathAssessmentVersion,"in_progress","{}","[]").Error; err != nil { t.Fatalf("seed School B attempt: %v",err) }

	parsed, err := url.Parse(dsn); if err != nil || parsed.Scheme == "" { t.Fatalf("TEST_DATABASE_URL must be a PostgreSQL URL: %v",err) }; parsed.User = url.UserPassword(role,password)
	restricted, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{}); if err != nil { t.Fatalf("connect restricted role: %v",err) }
	restrictedSQL, err := restricted.DB(); if err != nil { t.Fatal(err) }; restrictedSQL.SetMaxOpenConns(1); t.Cleanup(func(){ _ = restrictedSQL.Close() })
	if err := restricted.Exec(fmt.Sprintf("SET search_path TO \"%s\", public",schema)).Error; err != nil { t.Fatal(err) }
	var flags struct{ Superuser bool; BypassRLS bool }
	if err := restricted.Raw("SELECT rolsuper AS superuser, rolbypassrls AS bypass_rls FROM pg_roles WHERE rolname=current_user").Scan(&flags).Error; err != nil { t.Fatal(err) }
	if flags.Superuser || flags.BypassRLS { t.Fatalf("test role is privileged: superuser=%t bypassrls=%t",flags.Superuser,flags.BypassRLS) }

	gin.SetMode(gin.TestMode)
	controller := NewCareerPathController(service.NewCareerPathService(restricted))
	router := gin.New()
	router.Use(func(c *gin.Context){
		schoolID,userID := schoolA,userA
		if c.GetHeader("X-Test-School") == "B" { schoolID,userID = schoolB,userB }
		c.Set(middleware.UserIDKey,userID.String()); c.Set(middleware.SchoolIDKey,schoolID.String()); c.Next()
	})
	router.Use(middleware.TenantTransaction(restricted))
	router.POST("/attempts",controller.CreateOrResume)
	router.GET("/attempts/:id",controller.Get)
	router.PUT("/attempts/:id/responses",controller.SaveResponses)

	request := func(method,path,school,body string)(int,[]byte){ req:=httptest.NewRequest(method,path,bytes.NewBufferString(body)); if school!="" { req.Header.Set("X-Test-School",school) }; rec:=httptest.NewRecorder(); router.ServeHTTP(rec,req); return rec.Code,rec.Body.Bytes() }
	status,body := request(http.MethodPost,"/attempts","A","")
	if status != http.StatusOK { t.Fatalf("School A create/resume failed: %d %s",status,body) }
	var created struct{ ID uuid.UUID `json:"id"`; Status string `json:"status"` }
	if err := json.Unmarshal(body,&created); err != nil { t.Fatalf("decode created attempt: %v",err) }
	if created.ID == uuid.Nil || created.Status != "in_progress" { t.Fatalf("unexpected School A attempt: %+v",created) }

	status,body = request(http.MethodGet,"/attempts/"+attemptB.String(),"A","")
	if status != http.StatusNotFound { t.Fatalf("School A must not read School B attempt; got %d %s",status,body) }
	status,body = request(http.MethodGet,"/attempts/"+attemptB.String(),"B","")
	if status != http.StatusOK { t.Fatalf("School B should read its own attempt after pooled connection reuse: %d %s",status,body) }
	status,body = request(http.MethodGet,"/attempts/"+created.ID.String(),"A","")
	if status != http.StatusOK { t.Fatalf("School A context must be restored after School B request: %d %s",status,body) }
	status,body = request(http.MethodPut,"/attempts/"+created.ID.String()+"/responses","A",`{"responses":{"interests":["build"]}}`)
	if status != http.StatusOK { t.Fatalf("School A should update its own attempt: %d %s",status,body) }
	var persisted string
	if err := admin.Raw("SELECT responses FROM \""+schema+"\".career_path_attempts WHERE id = ?",created.ID).Scan(&persisted).Error; err != nil { t.Fatal(err) }
	if !strings.Contains(persisted,`"build"`) { t.Fatalf("CareerPath response was not persisted: %s",persisted) }
}
