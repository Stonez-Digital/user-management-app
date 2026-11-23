package middleware

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestRequireSchoolContextSetsSchoolID(t *testing.T) {
    db,err:=gorm.Open(sqlite.Open("file::memory:?cache=shared"),&gorm.Config{});if err!=nil{t.Fatal(err)}
    if err:=db.AutoMigrate(&models.School{},&models.User{});err!=nil{t.Fatal(err)}
    school:=models.School{Name:"Test School",Code:"TEST"};if err:=db.Create(&school).Error;err!=nil{t.Fatal(err)}
    user:=models.User{Name:"Admin",Email:"admin@test.example",Role:"school_admin",Active:true,SchoolID:&school.ID};if err:=db.Create(&user).Error;err!=nil{t.Fatal(err)}
    gin.SetMode(gin.TestMode);r:=gin.New();r.Use(func(c *gin.Context){c.Set(UserIDKey,user.ID.String());c.Next()});r.Use(RequireSchoolContext(db));r.GET("/",func(c *gin.Context){id,ok:=SchoolIDFromContext(c);if !ok||id!=school.ID{c.Status(http.StatusInternalServerError);return};c.Status(http.StatusNoContent)})
    req:=httptest.NewRequest(http.MethodGet,"/",nil);rec:=httptest.NewRecorder();r.ServeHTTP(rec,req);if rec.Code!=http.StatusNoContent{t.Fatalf("expected 204, got %d",rec.Code)}
}

func TestRequireSchoolContextRejectsUserWithoutSchool(t *testing.T) {
    db,err:=gorm.Open(sqlite.Open("file::memory:?cache=shared"),&gorm.Config{});if err!=nil{t.Fatal(err)}
    if err:=db.AutoMigrate(&models.School{},&models.User{});err!=nil{t.Fatal(err)}
    user:=models.User{Name:"Admin",Email:"noschool@test.example",Role:"school_admin",Active:true};if err:=db.Create(&user).Error;err!=nil{t.Fatal(err)}
    gin.SetMode(gin.TestMode);r:=gin.New();r.Use(func(c *gin.Context){c.Set(UserIDKey,user.ID.String());c.Next()});r.Use(RequireSchoolContext(db));r.GET("/",func(c *gin.Context){c.Status(http.StatusNoContent)})
    req:=httptest.NewRequest(http.MethodGet,"/",nil);rec:=httptest.NewRecorder();r.ServeHTTP(rec,req);if rec.Code!=http.StatusForbidden{t.Fatalf("expected 403, got %d",rec.Code)}
}

var _ = uuid.Nil


func TestTenantRollbackCleanupRunsWhenRequestRollsBack(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant-rollback-cleanup?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	schoolID, userID := uuid.New(), uuid.New()
	cleanupCalls := 0

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, userID.String())
		c.Set(SchoolIDKey, schoolID.String())
		c.Next()
	})
	r.Use(TenantTransaction(db))
	r.GET("/rollback", func(c *gin.Context) {
		if err := RegisterTenantRollbackCleanup(c, func() error { cleanupCalls++; return nil }); err != nil { t.Fatal(err) }
		c.Status(http.StatusBadRequest)
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/rollback", nil))
	if rec.Code != http.StatusBadRequest { t.Fatalf("expected 400, got %d", rec.Code) }
	if cleanupCalls != 1 { t.Fatalf("expected rollback cleanup once, got %d", cleanupCalls) }
}

func TestTenantRollbackCleanupDoesNotRunAfterCommit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:tenant-commit-cleanup?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil { t.Fatal(err) }
	schoolID, userID := uuid.New(), uuid.New()
	cleanupCalls := 0

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(UserIDKey, userID.String())
		c.Set(SchoolIDKey, schoolID.String())
		c.Next()
	})
	r.Use(TenantTransaction(db))
	r.GET("/commit", func(c *gin.Context) {
		if err := RegisterTenantRollbackCleanup(c, func() error { cleanupCalls++; return nil }); err != nil { t.Fatal(err) }
		c.Status(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/commit", nil))
	if rec.Code != http.StatusNoContent { t.Fatalf("expected 204, got %d", rec.Code) }
	if cleanupCalls != 0 { t.Fatalf("cleanup must not run after commit, got %d calls", cleanupCalls) }
}
