package controller

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "testing"

 "github.com/gin-gonic/gin"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/middleware"
 "github.com/onoja217/users-management-app/internal/models"
 "github.com/onoja217/users-management-app/internal/repository"
 "github.com/onoja217/users-management-app/internal/service"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
)

func schoolContentTenantDB(t *testing.T) *gorm.DB {
 t.Helper()
 db, err := gorm.Open(sqlite.Open("file:school_content_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
 if err != nil { t.Fatal(err) }
 if err := db.AutoMigrate(&models.School{}, &models.BlogPost{}, &models.GalleryAlbum{}, &models.GalleryImage{}, &models.SchoolAdvertisement{}, &models.SchoolEnquiry{}); err != nil { t.Fatal(err) }
 return db
}

func schoolContentTenantRequest(db *gorm.DB, schoolID, userID uuid.UUID, withTransaction bool, path string, handler gin.HandlerFunc) *httptest.ResponseRecorder {
 gin.SetMode(gin.TestMode)
 router := gin.New()
 router.Use(func(c *gin.Context) {
  c.Set(middleware.SchoolIDKey, schoolID.String())
  c.Set(middleware.UserIDKey, userID.String())
  c.Next()
 })
 if withTransaction { router.Use(middleware.TenantTransaction(db)) }
 router.GET(path, handler)
 recorder := httptest.NewRecorder()
 router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
 return recorder
}

func TestSchoolContentControllerReadsOnlyTheRequestTenant(t *testing.T) {
 db := schoolContentTenantDB(t)
 schoolA := models.School{Name:"Content A", Code:"CONTENT-A"}
 schoolB := models.School{Name:"Content B", Code:"CONTENT-B"}
 if err:=db.Create(&schoolA).Error; err!=nil { t.Fatal(err) }
 if err:=db.Create(&schoolB).Error; err!=nil { t.Fatal(err) }
 postA:=models.BlogPost{ID:uuid.New(),SchoolID:schoolA.ID,AuthorID:uuid.New(),Title:"A post",Slug:"a-post",Content:"A",Status:models.ContentDraft}
 postB:=models.BlogPost{ID:uuid.New(),SchoolID:schoolB.ID,AuthorID:uuid.New(),Title:"B post",Slug:"b-post",Content:"B",Status:models.ContentDraft}
 if err:=db.Create(&postA).Error; err!=nil { t.Fatal(err) }
 if err:=db.Create(&postB).Error; err!=nil { t.Fatal(err) }
 ctrl:=NewSchoolContentController(service.NewSchoolContentService(db),nil,db)
 response:=schoolContentTenantRequest(db,schoolA.ID,uuid.New(),true,"/blogs",ctrl.ListBlogs)
 if response.Code!=http.StatusOK { t.Fatalf("expected 200, got %d: %s",response.Code,response.Body.String()) }
 var payload struct { Posts []models.BlogPost `json:"posts"` }
 if err:=json.Unmarshal(response.Body.Bytes(),&payload); err!=nil { t.Fatal(err) }
 if len(payload.Posts)!=1 || payload.Posts[0].ID!=postA.ID { t.Fatalf("school A must only see its own blog post, got %+v",payload.Posts) }
}

func TestSchoolContentControllerFailsClosedWithoutTenantTransaction(t *testing.T) {
 db:=schoolContentTenantDB(t)
 school:=models.School{Name:"Content Missing Tx",Code:"CONTENT-NO-TX"}
 if err:=db.Create(&school).Error; err!=nil { t.Fatal(err) }
 ctrl:=NewSchoolContentController(service.NewSchoolContentService(db),nil,db)
 response:=schoolContentTenantRequest(db,school.ID,uuid.New(),false,"/blogs",ctrl.ListBlogs)
 if response.Code!=http.StatusInternalServerError { t.Fatalf("expected missing transaction to fail closed with 500, got %d: %s",response.Code,response.Body.String()) }
}

func TestSchoolProfileControllerFailsClosedWithoutTenantTransaction(t *testing.T) {
 db:=schoolContentTenantDB(t)
 school:=models.School{Name:"Profile Missing Tx",Code:"PROFILE-NO-TX"}
 if err:=db.Create(&school).Error; err!=nil { t.Fatal(err) }
 ctrl:=NewSchoolController(service.NewSchoolService(repository.NewSchoolRepository(db),db),nil)
 response:=schoolContentTenantRequest(db,school.ID,uuid.New(),false,"/school",ctrl.Get)
 if response.Code!=http.StatusInternalServerError { t.Fatalf("expected missing transaction to fail closed with 500, got %d: %s",response.Code,response.Body.String()) }
}

func TestEverySchoolAdminContentHandlerFailsClosedWithoutTenantTransaction(t *testing.T) {
 db := schoolContentTenantDB(t)
 school := models.School{Name: "All Handler Guard", Code: "ALL-HANDLER-GUARD"}
 if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
 content := NewSchoolContentController(service.NewSchoolContentService(db), nil, db)
 profile := NewSchoolController(service.NewSchoolService(repository.NewSchoolRepository(db), db), nil)

 handlers := []struct {
  name string
  handler gin.HandlerFunc
 }{
  {"list advertisements", content.ListAdvertisements},
  {"create advertisement", content.CreateAdvertisement},
  {"update advertisement", content.UpdateAdvertisement},
  {"delete advertisement", content.DeleteAdvertisement},
  {"upload advertisement image", content.UploadAdvertisementImage},
  {"list blogs", content.ListBlogs},
  {"create blog", content.CreateBlog},
  {"update blog", content.UpdateBlog},
  {"delete blog", content.DeleteBlog},
  {"publish blog", content.PublishBlog},
  {"list albums", content.ListAlbums},
  {"create album", content.CreateAlbum},
  {"update album", content.UpdateAlbum},
  {"delete album", content.DeleteAlbum},
  {"upload gallery image", content.UploadImage},
  {"delete gallery image", content.DeleteImage},
  {"upload blog image", content.UploadBlogImage},
  {"list enquiries", content.ListEnquiries},
  {"update enquiry status", content.UpdateEnquiryStatus},
  {"delete enquiry", content.DeleteEnquiry},
  {"get school profile", profile.Get},
  {"update school profile", profile.Update},
  {"upload school logo", profile.UploadLogo},
 }

 for _, test := range handlers {
  t.Run(test.name, func(t *testing.T) {
   response := schoolContentTenantRequest(db, school.ID, uuid.New(), false, "/guard", test.handler)
   if response.Code != http.StatusInternalServerError {
    t.Fatalf("handler must reject requests without a tenant transaction: got %d: %s", response.Code, response.Body.String())
   }
   var payload map[string]interface{}
   if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil { t.Fatalf("expected JSON error response: %v", err) }
   errorBody, ok := payload["error"].(map[string]interface{})
   if !ok || errorBody["code"] != "tenant_transaction_required" {
    t.Fatalf("expected tenant_transaction_required, got %v", payload["error"])
   }
  })
 }
}
