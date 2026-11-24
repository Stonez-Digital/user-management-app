package controller

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"

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
 if err := db.AutoMigrate(&models.School{}, &models.BlogPost{}, &models.GalleryAlbum{}, &models.GalleryImage{}, &models.SchoolAdvertisement{}, &models.SchoolEnquiry{}, &models.AuditLog{}); err != nil { t.Fatal(err) }
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

func TestSchoolContentListsAdvertisementsAndEnquiriesOnlyForRequestTenant(t *testing.T) {
 db := schoolContentTenantDB(t)
 schoolA := models.School{Name: "Tenant A", Code: "LIST-TENANT-A"}
 schoolB := models.School{Name: "Tenant B", Code: "LIST-TENANT-B"}
 if err := db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
 if err := db.Create(&schoolB).Error; err != nil { t.Fatal(err) }

 adA := models.SchoolAdvertisement{ID: uuid.New(), SchoolID: schoolA.ID, Title: "A advert", ImageURL: "https://media.invalid/a.png", Status: models.ContentDraft, CreatedBy: uuid.New()}
 adB := models.SchoolAdvertisement{ID: uuid.New(), SchoolID: schoolB.ID, Title: "B advert", ImageURL: "https://media.invalid/b.png", Status: models.ContentDraft, CreatedBy: uuid.New()}
 for _, ad := range []models.SchoolAdvertisement{adA, adB} {
  if err := db.Create(&ad).Error; err != nil { t.Fatal(err) }
 }
 enquiryA := models.SchoolEnquiry{ID: uuid.New(), SchoolID: schoolA.ID, FullName: "A Contact", Email: "a@example.test", Subject: "General", Message: "A private enquiry", Status: models.SchoolEnquiryNew, ConsentAt: time.Now().UTC()}
 enquiryB := models.SchoolEnquiry{ID: uuid.New(), SchoolID: schoolB.ID, FullName: "B Contact", Email: "b@example.test", Subject: "General", Message: "B private enquiry", Status: models.SchoolEnquiryNew, ConsentAt: time.Now().UTC()}
 for _, enquiry := range []models.SchoolEnquiry{enquiryA, enquiryB} {
  if err := db.Create(&enquiry).Error; err != nil { t.Fatal(err) }
 }

 ctrl := NewSchoolContentController(service.NewSchoolContentService(db), nil, db)
 adResponse := schoolContentTenantRequest(db, schoolA.ID, uuid.New(), true, "/advertisements", ctrl.ListAdvertisements)
 if adResponse.Code != http.StatusOK { t.Fatalf("advertisement list failed: %d %s", adResponse.Code, adResponse.Body.String()) }
 var adPayload struct { Advertisements []models.SchoolAdvertisement `json:"advertisements"` }
 if err := json.Unmarshal(adResponse.Body.Bytes(), &adPayload); err != nil { t.Fatal(err) }
 if len(adPayload.Advertisements) != 1 || adPayload.Advertisements[0].ID != adA.ID {
  t.Fatalf("school A must only see its own advertisements, got %+v", adPayload.Advertisements)
 }

 enquiryResponse := schoolContentTenantRequest(db, schoolA.ID, uuid.New(), true, "/enquiries", ctrl.ListEnquiries)
 if enquiryResponse.Code != http.StatusOK { t.Fatalf("enquiry list failed: %d %s", enquiryResponse.Code, enquiryResponse.Body.String()) }
 var enquiryPayload struct { Enquiries []models.SchoolEnquiry `json:"enquiries"` }
 if err := json.Unmarshal(enquiryResponse.Body.Bytes(), &enquiryPayload); err != nil { t.Fatal(err) }
 if len(enquiryPayload.Enquiries) != 1 || enquiryPayload.Enquiries[0].ID != enquiryA.ID {
  t.Fatalf("school A must only see its own enquiries, got %+v", enquiryPayload.Enquiries)
 }
}


func TestBlogMutationRollsBackWhenAuditWriteFails(t *testing.T) {
 db := schoolContentTenantDB(t)
 school := models.School{Name: "Audit rollback", Code: "AUDIT-ROLLBACK"}
 if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
 post := models.BlogPost{ID: uuid.New(), SchoolID: school.ID, AuthorID: uuid.New(), Title: "Must remain", Slug: "must-remain", Content: "body", Status: models.ContentDraft}
 if err := db.Create(&post).Error; err != nil { t.Fatal(err) }
 if err := db.Exec(`CREATE TRIGGER reject_content_audit BEFORE INSERT ON audit_logs BEGIN SELECT RAISE(FAIL, 'audit unavailable'); END`).Error; err != nil { t.Fatal(err) }

 ctrl := NewSchoolContentController(service.NewSchoolContentService(db), nil, db)
 gin.SetMode(gin.TestMode)
 router := gin.New()
 router.Use(func(c *gin.Context) {
  c.Set(middleware.SchoolIDKey, school.ID.String())
  c.Set(middleware.UserIDKey, uuid.NewString())
  c.Next()
 })
 router.Use(middleware.TenantTransaction(db))
 router.DELETE("/blogs/:id", ctrl.DeleteBlog)
 response := httptest.NewRecorder()
 request := httptest.NewRequest(http.MethodDelete, "/blogs/"+post.ID.String(), nil)
 router.ServeHTTP(response, request)
 if response.Code != http.StatusInternalServerError {
  t.Fatalf("expected audit failure to return 500, got %d: %s", response.Code, response.Body.String())
 }
 var count int64
 if err := db.Model(&models.BlogPost{}).Where("school_id = ? AND id = ?", school.ID, post.ID).Count(&count).Error; err != nil { t.Fatal(err) }
 if count != 1 { t.Fatalf("blog deletion must roll back when its audit write fails; found %d rows", count) }
}


func TestSchoolAMutationsCannotReadOrChangeSchoolBContent(t *testing.T) {
 db := schoolContentTenantDB(t)
 schoolA := models.School{Name: "Mutation A", Code: "MUTATION-A"}
 schoolB := models.School{Name: "Mutation B", Code: "MUTATION-B"}
 if err := db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
 if err := db.Create(&schoolB).Error; err != nil { t.Fatal(err) }

 blogB := models.BlogPost{ID: uuid.New(), SchoolID: schoolB.ID, AuthorID: uuid.New(), Title: "B blog", Slug: "b-blog", Content: "private", Status: models.ContentDraft}
 albumB := models.GalleryAlbum{ID: uuid.New(), SchoolID: schoolB.ID, Title: "B album", Slug: "b-album", Status: models.ContentDraft}
 adB := models.SchoolAdvertisement{ID: uuid.New(), SchoolID: schoolB.ID, Title: "B advert", ImageURL: "https://media.invalid/b.png", Status: models.ContentDraft, CreatedBy: uuid.New()}
 enquiryB := models.SchoolEnquiry{ID: uuid.New(), SchoolID: schoolB.ID, FullName: "B Contact", Email: "b@example.test", Subject: "General", Message: "Private school B enquiry", Status: models.SchoolEnquiryNew, ConsentAt: time.Now().UTC()}
 for _, record := range []interface{}{&blogB, &albumB, &adB, &enquiryB} {
  if err := db.Create(record).Error; err != nil { t.Fatal(err) }
 }
 imageB := models.GalleryImage{ID: uuid.New(), SchoolID: schoolB.ID, AlbumID: albumB.ID, UploadedBy: uuid.New(), ImageURL: "https://media.invalid/b-gallery.png"}
 if err := db.Create(&imageB).Error; err != nil { t.Fatal(err) }

 ctrl := NewSchoolContentController(service.NewSchoolContentService(db), nil, db)
 userID := uuid.New()
 gin.SetMode(gin.TestMode)
 router := gin.New()
 router.Use(func(c *gin.Context) {
  c.Set(middleware.SchoolIDKey, schoolA.ID.String())
  c.Set(middleware.UserIDKey, userID.String())
  c.Next()
 })
 router.Use(middleware.TenantTransaction(db))
 router.PUT("/blogs/:id", ctrl.UpdateBlog)
 router.DELETE("/blogs/:id", ctrl.DeleteBlog)
 router.PUT("/albums/:id", ctrl.UpdateAlbum)
 router.DELETE("/albums/:id", ctrl.DeleteAlbum)
 router.DELETE("/images/:id", ctrl.DeleteImage)
 router.PUT("/advertisements/:id", ctrl.UpdateAdvertisement)
 router.DELETE("/advertisements/:id", ctrl.DeleteAdvertisement)
 router.PATCH("/enquiries/:id", ctrl.UpdateEnquiryStatus)
 router.DELETE("/enquiries/:id", ctrl.DeleteEnquiry)

 cases := []struct{method, path, body string}{
  {http.MethodPut, "/blogs/"+blogB.ID.String(), `{"title":"tampered"}`},
  {http.MethodDelete, "/blogs/"+blogB.ID.String(), ""},
  {http.MethodPut, "/albums/"+albumB.ID.String(), `{"title":"tampered"}`},
  {http.MethodDelete, "/albums/"+albumB.ID.String(), ""},
  {http.MethodDelete, "/images/"+imageB.ID.String(), ""},
  {http.MethodPut, "/advertisements/"+adB.ID.String(), `{"title":"tampered"}`},
  {http.MethodDelete, "/advertisements/"+adB.ID.String(), ""},
  {http.MethodPatch, "/enquiries/"+enquiryB.ID.String(), `{"status":"closed"}`},
  {http.MethodDelete, "/enquiries/"+enquiryB.ID.String(), ""},
 }
 for _, tc := range cases {
  request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
  response := httptest.NewRecorder()
  router.ServeHTTP(response, request)
  if response.Code != http.StatusNotFound {
   t.Errorf("%s %s should not find school B's record, got %d: %s", tc.method, tc.path, response.Code, response.Body.String())
  }
 }

 var blog models.BlogPost
 if err := db.Where("id = ?", blogB.ID).First(&blog).Error; err != nil { t.Fatalf("school B blog was changed or deleted: %v", err) }
 if blog.Title != "B blog" { t.Fatalf("school B blog was modified: %q", blog.Title) }
 var album models.GalleryAlbum
 if err := db.Where("id = ?", albumB.ID).First(&album).Error; err != nil { t.Fatalf("school B album was changed or deleted: %v", err) }
 var imageCount, adCount, enquiryCount, auditCount int64
 if err := db.Model(&models.GalleryImage{}).Where("id = ?", imageB.ID).Count(&imageCount).Error; err != nil { t.Fatal(err) }
 if err := db.Model(&models.SchoolAdvertisement{}).Where("id = ?", adB.ID).Count(&adCount).Error; err != nil { t.Fatal(err) }
 if err := db.Model(&models.SchoolEnquiry{}).Where("id = ?", enquiryB.ID).Count(&enquiryCount).Error; err != nil { t.Fatal(err) }
 if err := db.Model(&models.AuditLog{}).Count(&auditCount).Error; err != nil { t.Fatal(err) }
 if imageCount != 1 || adCount != 1 || enquiryCount != 1 { t.Fatalf("school B data was changed: image=%d ad=%d enquiry=%d", imageCount, adCount, enquiryCount) }
 if auditCount != 0 { t.Fatalf("rejected cross-tenant mutations must not leave audit writes, got %d", auditCount) }
}

func TestSchoolContentAuditUsesRequestTenant(t *testing.T) {
 db := schoolContentTenantDB(t)
 schoolA := models.School{Name: "Audit Tenant A", Code: "AUDIT-TENANT-A"}
 schoolB := models.School{Name: "Audit Tenant B", Code: "AUDIT-TENANT-B"}
 if err := db.Create(&schoolA).Error; err != nil { t.Fatal(err) }
 if err := db.Create(&schoolB).Error; err != nil { t.Fatal(err) }
 ctrl := NewSchoolContentController(service.NewSchoolContentService(db), nil, db)
 userID := uuid.New()
 gin.SetMode(gin.TestMode)
 router := gin.New()
 router.Use(func(c *gin.Context) {
  c.Set(middleware.SchoolIDKey, schoolA.ID.String())
  c.Set(middleware.UserIDKey, userID.String())
  c.Next()
 })
 router.Use(middleware.TenantTransaction(db))
 router.POST("/blogs", ctrl.CreateBlog)
 request := httptest.NewRequest(http.MethodPost, "/blogs", strings.NewReader(`{"title":"Tenant A update","content":"owned by A"}`))
 response := httptest.NewRecorder()
 router.ServeHTTP(response, request)
 if response.Code != http.StatusCreated { t.Fatalf("blog create failed: %d %s", response.Code, response.Body.String()) }
 var logs []models.AuditLog
 if err := db.Where("action = ?", "content.blog.create").Find(&logs).Error; err != nil { t.Fatal(err) }
 if len(logs) != 1 || logs[0].SchoolID == nil || *logs[0].SchoolID != schoolA.ID {
  t.Fatalf("audit must be written for request tenant A only, got %+v", logs)
 }
}
