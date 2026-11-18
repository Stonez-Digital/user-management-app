package service

import (
 "testing"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
)

func contentTestDB(t *testing.T)*gorm.DB{
 t.Helper()
 db,e:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if e!=nil{t.Fatal(e)}
 if e=db.AutoMigrate(&models.School{},&models.User{},&models.BlogPost{},&models.BlogCategory{},&models.GalleryAlbum{},&models.GalleryImage{});e!=nil{t.Fatal(e)}
 return db
}

func TestSchoolContentIsTenantScoped(t *testing.T){
 db:=contentTestDB(t);svc:=NewSchoolContentService(db)
 a:=models.School{Name:"School A",Code:"A"};b:=models.School{Name:"School B",Code:"B"};if e:=db.Create(&a).Error;e!=nil{t.Fatal(e)};if e:=db.Create(&b).Error;e!=nil{t.Fatal(e)}
 authorA:=models.User{Name:"Admin A",Email:"a-"+uuid.NewString()+"@test",Role:"school_admin",Active:true,SchoolID:&a.ID};authorB:=models.User{Name:"Admin B",Email:"b-"+uuid.NewString()+"@test",Role:"school_admin",Active:true,SchoolID:&b.ID};if e:=db.Create(&authorA).Error;e!=nil{t.Fatal(e)};if e:=db.Create(&authorB).Error;e!=nil{t.Fatal(e)}
 p,e:=svc.CreateBlog(a.ID,authorA.ID,&models.BlogPost{Title:"Graduation",Content:"A",Status:models.ContentPublished});if e!=nil{t.Fatal(e)}
 if _,e=svc.CreateBlog(b.ID,authorB.ID,&models.BlogPost{Title:"Graduation",Content:"B",Status:models.ContentPublished});e!=nil{t.Fatal(e)}
 posts,e:=svc.PublicBlogs(a.ID);if e!=nil||len(posts)!=1||posts[0].Content!="A"{t.Fatalf("school A isolation failed: %d %v",len(posts),e)}
 if _,e=svc.PublicBlog(b.ID,p.Slug);e!=nil{t.Fatalf("same slug must be valid in another school: %v",e)}
 if _,e=svc.PublicBlog(a.ID,"missing");e!=ErrContentNotFound{t.Fatalf("expected not found, got %v",e)}
 album,e:=svc.CreateAlbum(a.ID,&models.GalleryAlbum{Title:"Events",Status:models.ContentPublished});if e!=nil{t.Fatal(e)}
 if _,e=svc.AddImage(b.ID,authorB.ID,album.ID,&models.GalleryImage{ImageURL:"https://example.invalid/x.webp"});e!=ErrContentNotFound{t.Fatalf("cross-school album write should fail, got %v",e)}
 if e=svc.DeleteAlbum(b.ID,album.ID);e!=nil{t.Fatal(e)}
 var count int64;if e=db.Model(&models.GalleryAlbum{}).Where("id=?",album.ID).Count(&count).Error;e!=nil{t.Fatal(e)};if count!=1{t.Fatal("school B must not delete school A album")}
}
