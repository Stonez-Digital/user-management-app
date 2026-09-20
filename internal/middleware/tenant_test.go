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
