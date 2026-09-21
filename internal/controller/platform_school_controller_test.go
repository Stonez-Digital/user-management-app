package controller

import(
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "github.com/gin-gonic/gin"
 "github.com/onoja217/users-management-app/internal/auth"
 "github.com/onoja217/users-management-app/internal/authz"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
)
func TestPlatformSchoolCreateProvisionsTenantAndAdmin(t *testing.T){
 db,err:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if err!=nil{t.Fatal(err)}
 if err=db.AutoMigrate(&models.School{},&models.User{});err!=nil{t.Fatal(err)}
 actorSchool:=models.School{Name:"Platform",Code:"PLATFORM",Status:models.SchoolStatusActive};if err=db.Create(&actorSchool).Error;err!=nil{t.Fatal(err)}
 actor:=models.User{Name:"Platform Admin",Email:"platform-"+t.Name()+"@example.com",Role:authz.RoleSuperAdmin,Active:true,SchoolID:&actorSchool.ID};if err=db.Create(&actor).Error;err!=nil{t.Fatal(err)}
 gin.SetMode(gin.TestMode);r:=gin.New();r.Use(func(c *gin.Context){c.Set("user_id",actor.ID.String());c.Next()});r.POST("/schools",NewPlatformSchoolController(db).Create)
 email:="admin-"+t.Name()+"@example.com";body:=strings.NewReader("{"name":"School A","code":"SCA-001","admin_name":"School Admin","admin_email":""+email+"","admin_password":"StrongPass123!"}")
 req:=httptest.NewRequest(http.MethodPost,"/schools",body);req.Header.Set("Content-Type","application/json");w:=httptest.NewRecorder();r.ServeHTTP(w,req)
 if w.Code!=http.StatusCreated{t.Fatalf("expected 201, got %d: %s",w.Code,w.Body.String())}
 var school models.School;if err=db.Where("code = ?","SCA-001").First(&school).Error;err!=nil{t.Fatal(err)}
 var admin models.User;if err=db.Where("email = ?",email).First(&admin).Error;err!=nil{t.Fatal(err)}
 if admin.SchoolID==nil||*admin.SchoolID!=school.ID||admin.Role!=authz.RoleSchoolAdmin||!auth.CheckPassword(admin.PasswordHash,"StrongPass123!"){t.Fatal("provisioned administrator is invalid")}
}
