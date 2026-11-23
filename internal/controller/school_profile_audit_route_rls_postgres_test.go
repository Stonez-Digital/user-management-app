package controller

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/middleware"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/repository"
	"github.com/onoja217/users-management-app/internal/service"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Exercises real school profile, logo and school audit handlers with PostgreSQL
// RLS forced under a restricted role. All objects live in a disposable schema.
func TestSchoolProfileLogoAndAuditRoutesEnforceTwoSchoolRLS(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		if strings.EqualFold(os.Getenv("CI"), "true") { t.Fatal("TEST_DATABASE_URL is required in CI; refusing to skip school profile/audit route RLS test") }
		t.Skip("TEST_DATABASE_URL is not configured outside CI")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil { t.Fatalf("open PostgreSQL test database: %v",err) }
	adminSQL, err := admin.DB(); if err != nil { t.Fatal(err) }; adminSQL.SetMaxOpenConns(1); t.Cleanup(func(){ _ = adminSQL.Close() })

	suffix := strings.ReplaceAll(uuid.NewString(),"-","")
	schema,role := "school_profile_rls_"+suffix,"school_profile_rls_"+suffix
	password := strings.ReplaceAll(uuid.NewString(),"-","")
	schoolA,schoolB,userA,userB := uuid.New(),uuid.New(),uuid.New(),uuid.New()
	if err := admin.Exec(fmt.Sprintf("CREATE ROLE \"%s\" LOGIN PASSWORD '%s' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS",role,password)).Error; err != nil { t.Fatalf("create restricted role: %v",err) }
	if err := admin.Exec(fmt.Sprintf("CREATE SCHEMA \"%s\"",schema)).Error; err != nil { _ = admin.Exec(fmt.Sprintf("DROP ROLE \"%s\"",role)).Error; t.Fatalf("create schema: %v",err) }
	t.Cleanup(func(){ _ = admin.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS \"%s\" CASCADE",schema)).Error; _ = admin.Exec(fmt.Sprintf("DROP OWNED BY \"%s\"",role)).Error; _ = admin.Exec(fmt.Sprintf("DROP ROLE IF EXISTS \"%s\"",role)).Error })
	if err := admin.Exec(fmt.Sprintf("SET search_path TO \"%s\", public",schema)).Error; err != nil { t.Fatal(err) }
	if err := admin.AutoMigrate(&models.School{},&models.AuditLog{}); err != nil { t.Fatalf("migrate production School/AuditLog models: %v",err) }
	if err := admin.Create(&models.School{ID:schoolA,Name:"School A",Code:"RLS-A",Status:models.SchoolStatusActive}).Error; err != nil { t.Fatal(err) }
	if err := admin.Create(&models.School{ID:schoolB,Name:"School B",Code:"RLS-B",Status:models.SchoolStatusActive}).Error; err != nil { t.Fatal(err) }
	for _,log := range []models.AuditLog{
		{ID:uuid.New(),SchoolID:&schoolA,ActorID:&userA,Action:"school_a.seed",Resource:"school"},
		{ID:uuid.New(),SchoolID:&schoolB,ActorID:&userB,Action:"school_b.seed",Resource:"school"},
		{ID:uuid.New(),SchoolID:nil,ActorID:nil,Action:"platform.seed",Resource:"platform"},
	} { if err := admin.Create(&log).Error; err != nil { t.Fatalf("seed audit log: %v",err) } }
	for _,stmt := range []string{
		"ALTER TABLE schools ENABLE ROW LEVEL SECURITY",
		"ALTER TABLE schools FORCE ROW LEVEL SECURITY",
		"CREATE POLICY school_isolation ON schools USING (id = nullif(current_setting('app.current_school_id',true),'')::uuid) WITH CHECK (id = nullif(current_setting('app.current_school_id',true),'')::uuid)",
		"ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY",
		"ALTER TABLE audit_logs FORCE ROW LEVEL SECURITY",
		"CREATE POLICY audit_school_isolation ON audit_logs USING (school_id = nullif(current_setting('app.current_school_id',true),'')::uuid) WITH CHECK (school_id = nullif(current_setting('app.current_school_id',true),'')::uuid)",
		fmt.Sprintf("GRANT USAGE ON SCHEMA \"%s\" TO \"%s\"",schema,role),
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE ON schools TO \"%s\"",role),
		fmt.Sprintf("GRANT SELECT, INSERT ON audit_logs TO \"%s\"",role),
	} { if err := admin.Exec(stmt).Error; err != nil { t.Fatalf("configure RLS: %v; SQL=%s",err,stmt) } }

	parsed,err:=url.Parse(dsn); if err!=nil||parsed.Scheme=="" { t.Fatalf("TEST_DATABASE_URL must be a PostgreSQL URL: %v",err) }; parsed.User=url.UserPassword(role,password)
	restricted,err:=gorm.Open(postgres.Open(parsed.String()),&gorm.Config{}); if err!=nil { t.Fatalf("connect restricted role: %v",err) }
	restrictedSQL,err:=restricted.DB(); if err!=nil { t.Fatal(err) }; restrictedSQL.SetMaxOpenConns(1); t.Cleanup(func(){ _=restrictedSQL.Close() })
	if err:=restricted.Exec(fmt.Sprintf("SET search_path TO \"%s\", public",schema)).Error; err!=nil { t.Fatal(err) }
	var flags struct{ Superuser bool; BypassRLS bool }
	if err:=restricted.Raw("SELECT rolsuper AS superuser, rolbypassrls AS bypass_rls FROM pg_roles WHERE rolname=current_user").Scan(&flags).Error; err!=nil { t.Fatal(err) }
	if flags.Superuser||flags.BypassRLS { t.Fatalf("test role is privileged: superuser=%t bypassrls=%t",flags.Superuser,flags.BypassRLS) }

	// Use a local HTTP storage stub so the real UploadLogo handler executes
	// without contacting an external storage provider.
	storageServer:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		switch {
		case r.Method==http.MethodPost && r.URL.Path=="/storage/v1/bucket": w.WriteHeader(http.StatusConflict); _,_=w.Write([]byte(`{"message":"BucketAlreadyExists"}`))
		case r.Method==http.MethodPost && strings.HasPrefix(r.URL.Path,"/storage/v1/object/school-logos/"): w.WriteHeader(http.StatusOK)
		case r.Method==http.MethodDelete && strings.HasPrefix(r.URL.Path,"/storage/v1/object/school-logos/"): w.WriteHeader(http.StatusOK)
		default: t.Errorf("unexpected storage request %s %s",r.Method,r.URL.Path); http.NotFound(w,r)
		}
	}))
	defer storageServer.Close()
	t.Setenv("SUPABASE_URL",storageServer.URL); t.Setenv("SUPABASE_SECRET_KEY","test-only-secret")
	logoStorage:=service.NewSchoolLogoStorageFromEnv()
	schoolController:=NewSchoolController(service.NewSchoolService(repository.NewSchoolRepository(restricted),restricted),logoStorage)
	auditController:=NewAuditController(restricted)
	gin.SetMode(gin.TestMode)
	router:=gin.New()
	router.Use(func(c *gin.Context){sid,uid:=schoolA,userA;if c.GetHeader("X-Test-School")=="B"{sid,uid=schoolB,userB};c.Set(middleware.SchoolIDKey,sid.String());c.Set(middleware.UserIDKey,uid.String());c.Next()})
	router.Use(middleware.TenantTransaction(restricted))
	router.GET("/school",schoolController.Get)
	router.PUT("/school",schoolController.Update)
	router.POST("/school/logo",schoolController.UploadLogo)
	router.GET("/audit-logs",auditController.List)
	request:=func(method,path,school,body string)(int,string){req:=httptest.NewRequest(method,path,strings.NewReader(body));if school!=""{req.Header.Set("X-Test-School",school)};rec:=httptest.NewRecorder();router.ServeHTTP(rec,req);return rec.Code,rec.Body.String()}

	status,body:=request(http.MethodGet,"/school","A","")
	if status!=http.StatusOK||!strings.Contains(body,"School A"){t.Fatalf("School A profile read failed: %d %s",status,body)}
	status,body=request(http.MethodPut,"/school","A",`{"name":"School A Updated","address":"A campus"}`)
	if status!=http.StatusOK||!strings.Contains(body,"School A Updated"){t.Fatalf("School A profile update failed: %d %s",status,body)}
	var schoolBAfter models.School
	if err:=admin.First(&schoolBAfter,"id = ?",schoolB).Error;err!=nil{t.Fatal(err)}
	if schoolBAfter.Name!="School B"{t.Fatalf("School A profile update changed School B: %+v",schoolBAfter)}

	var logoBody bytes.Buffer
	writer:=multipart.NewWriter(&logoBody)
	part,err:=writer.CreateFormFile("logo","school.png");if err!=nil{t.Fatal(err)}
	if _,err:=part.Write([]byte{0x89,'P','N','G',0x0d,0x0a,0x1a,0x0a});err!=nil{t.Fatal(err)}
	if err:=writer.Close();err!=nil{t.Fatal(err)}
	logoReq:=httptest.NewRequest(http.MethodPost,"/school/logo",&logoBody);logoReq.Header.Set("Content-Type",writer.FormDataContentType())
	logoRec:=httptest.NewRecorder();router.ServeHTTP(logoRec,logoReq)
	if logoRec.Code!=http.StatusOK{t.Fatalf("School A logo upload failed: %d %s",logoRec.Code,logoRec.Body.String())}
	var schoolAAfter,schoolBAfterLogo models.School
	if err:=admin.First(&schoolAAfter,"id = ?",schoolA).Error;err!=nil{t.Fatal(err)}
	if err:=admin.First(&schoolBAfterLogo,"id = ?",schoolB).Error;err!=nil{t.Fatal(err)}
	if !strings.Contains(schoolAAfter.LogoURL,schoolA.String()+"/"){t.Fatalf("School A logo URL not saved under its storage prefix: %q",schoolAAfter.LogoURL)}
	if schoolBAfterLogo.LogoURL!=""{t.Fatalf("School A logo operation changed School B logo: %q",schoolBAfterLogo.LogoURL)}

	status,body=request(http.MethodGet,"/audit-logs","A","")
	if status!=http.StatusOK||!strings.Contains(body,"school_a.seed"){t.Fatalf("School A audit list missing own event: %d %s",status,body)}
	if strings.Contains(body,"school_b.seed")||strings.Contains(body,"platform.seed"){t.Fatalf("school audit route leaked another school's or platform event: %s",body)}
	status,body=request(http.MethodGet,"/audit-logs","B","")
	if status!=http.StatusOK||!strings.Contains(body,"school_b.seed"){t.Fatalf("School B audit list missing own event after pool reuse: %d %s",status,body)}
	if strings.Contains(body,"school_a.seed")||strings.Contains(body,"platform.seed"){t.Fatalf("School B audit route leaked School A or platform event: %s",body)}
}
