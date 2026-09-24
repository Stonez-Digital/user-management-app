package controller

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/gin-gonic/gin"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/auth"
 "github.com/onoja217/users-management-app/internal/authz"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
)

func brandedLoginDB(t *testing.T) *gorm.DB {
 t.Helper()
 db,err:=gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"),&gorm.Config{})
 if err!=nil{t.Fatal(err)}
 if err=db.AutoMigrate(&models.User{},&models.RefreshToken{},&models.Session{});err!=nil{t.Fatal(err)}
 if err=db.Exec("CREATE TABLE schools (id text primary key, name text not null, code text not null, slug text, logo_url text, status text not null, created_at datetime, updated_at datetime)").Error;err!=nil{t.Fatal(err)}
 return db
}

func TestPublicSchoolReturnsSafeBranding(t *testing.T){
 db:=brandedLoginDB(t)
 school:=models.School{Name:"Bedrock International Academy",Code:"001",Slug:"bedrock-international-academy",Status:models.SchoolStatusActive,LogoURL:"https://example.com/bedrock.png"}
 if err:=db.Create(&school).Error;err!=nil{t.Fatal(err)}
 r:=gin.New();r.GET("/public/schools/:slug",NewAuthController(db).PublicSchool)
 req:=httptest.NewRequest(http.MethodGet,"/public/schools/bedrock-international-academy",nil);w:=httptest.NewRecorder();r.ServeHTTP(w,req)
 if w.Code!=http.StatusOK{t.Fatalf("expected 200, got %d: %s",w.Code,w.Body.String())}
 var body map[string]any
 if err:=json.Unmarshal(w.Body.Bytes(),&body);err!=nil{t.Fatal(err)}
 if body["name"]!="Bedrock International Academy"||body["logo_url"]!="https://example.com/bedrock.png"{t.Fatalf("unexpected branding response: %#v",body)}
 for _,key:=range []string{"password_hash","users","admin_email"}{if _,ok:=body[key];ok{t.Fatalf("public branding exposed %s",key)}}
}

func TestPublicSchoolRejectsInactiveSchool(t *testing.T){
 db:=brandedLoginDB(t)
 school:=models.School{Name:"Inactive School",Code:"INACTIVE",Slug:"inactive-school",Status:models.SchoolStatusSuspended}
 if err:=db.Create(&school).Error;err!=nil{t.Fatal(err)}
 r:=gin.New();r.GET("/public/schools/:slug",NewAuthController(db).PublicSchool)
 req:=httptest.NewRequest(http.MethodGet,"/public/schools/inactive-school",nil);w:=httptest.NewRecorder();r.ServeHTTP(w,req)
 if w.Code!=http.StatusGone{t.Fatalf("expected 410, got %d",w.Code)}
}

func TestSchoolLoginRejectsWrongTenantSlug(t *testing.T){
 db:=brandedLoginDB(t)
 if err:=auth.ConfigureSecret("test-secret-that-is-at-least-32-characters-long");err!=nil{t.Fatal(err)}
 hash,err:=auth.HashPassword("password123");if err!=nil{t.Fatal(err)}
 a:=models.School{Name:"School A",Code:"A",Slug:"school-a",Status:models.SchoolStatusActive}
 b:=models.School{Name:"School B",Code:"B",Slug:"school-b",Status:models.SchoolStatusActive}
 if err=db.Create(&a).Error;err!=nil{t.Fatal(err)};if err=db.Create(&b).Error;err!=nil{t.Fatal(err)}
 u:=models.User{Name:"Teacher A",Email:"teacher-a@example.com",PasswordHash:hash,Role:authz.RoleTeacher,Active:true,SchoolID:&a.ID}
 if err=db.Create(&u).Error;err!=nil{t.Fatal(err)}
 r:=gin.New();r.POST("/auth/login",NewAuthController(db).Login)
 body:=strings.NewReader("{\"email\":\"teacher-a@example.com\",\"password\":\"password123\",\"school_slug\":\"school-b\"}")
 req:=httptest.NewRequest(http.MethodPost,"/auth/login",body);req.Header.Set("Content-Type","application/json");w:=httptest.NewRecorder();r.ServeHTTP(w,req)
 if w.Code!=http.StatusUnauthorized{t.Fatalf("expected 401, got %d: %s",w.Code,w.Body.String())}
 if strings.Contains(strings.ToLower(w.Body.String()),"access_token"){t.Fatal("wrong-tenant login issued an access token")}
}
