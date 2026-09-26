package service

import (
 "errors"
 "strings"
 "time"
 "unicode"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/gorm"
)

var ErrContentNotFound=errors.New("content not found")
var ErrContentSlugExists=errors.New("content slug already exists")
var ErrContentInvalidStatus=errors.New("invalid content status")

type SchoolContentService struct{db *gorm.DB}
func NewSchoolContentService(db *gorm.DB)*SchoolContentService{return &SchoolContentService{db:db}}
func slugify(v string)string{v=strings.ToLower(strings.TrimSpace(v));var b strings.Builder;dash:=false;for _,r:=range v{if unicode.IsLetter(r)||unicode.IsDigit(r){b.WriteRune(r);dash=false}else if !dash{b.WriteByte('-');dash=true}};return strings.Trim(b.String(),"-")}
func validStatus(v string)bool{return v==models.ContentDraft||v==models.ContentPublished||v==models.ContentArchived}

func(s *SchoolContentService) CreateBlog(schoolID,authorID uuid.UUID,p *models.BlogPost)(models.BlogPost,error){
 p.ID=uuid.New();p.SchoolID=schoolID;p.AuthorID=authorID;p.Title=strings.TrimSpace(p.Title);p.Slug=slugify(p.Slug);if p.Slug==""{p.Slug=slugify(p.Title)};if p.Status==""{p.Status=models.ContentDraft};if !validStatus(p.Status){return *p,ErrContentInvalidStatus}
 var n int64;s.db.Model(&models.BlogPost{}).Where("school_id=? AND lower(slug)=lower(?)",schoolID,p.Slug).Count(&n);if n>0{return *p,ErrContentSlugExists}
 if p.Status==models.ContentPublished{now:=time.Now().UTC();p.PublishedAt=&now};if err:=s.db.Create(p).Error;err!=nil{return *p,err};return *p,nil
}
func(s *SchoolContentService) UpdateBlog(schoolID,id uuid.UUID,updates map[string]interface{})(models.BlogPost,error){
 var p models.BlogPost;if err:=s.db.Where("school_id=? AND id=?",schoolID,id).First(&p).Error;err!=nil{return p,ErrContentNotFound}
 allowed:=map[string]bool{"title":true,"slug":true,"excerpt":true,"content":true,"featured_image_url":true,"status":true,"featured":true}
 safe:=map[string]interface{}{};for k,v:=range updates{if allowed[k]{safe[k]=v}}
 if v,ok:=safe["slug"].(string);ok{safe["slug"]=slugify(v);if safe["slug"]==""{safe["slug"]=p.Slug};var n int64;s.db.Model(&models.BlogPost{}).Where("school_id=? AND lower(slug)=lower(?) AND id<>?",schoolID,safe["slug"],id).Count(&n);if n>0{return *p,ErrContentSlugExists}}
 if v,ok:=safe["status"].(string);ok{if !validStatus(v){return *p,ErrContentInvalidStatus};if v==models.ContentPublished&&p.PublishedAt==nil{safe["published_at"]=time.Now().UTC()};if v!=models.ContentPublished{safe["published_at"]=nil}}
 if err:=s.db.Model(&p).Updates(safe).Error;err!=nil{return p,err};if err:=s.db.Where("school_id=? AND id=?",schoolID,id).First(&p).Error;err!=nil{return p,err};return p,nil
}
func(s *SchoolContentService) DeleteBlog(schoolID,id uuid.UUID)error{return s.db.Where("school_id=? AND id=?",schoolID,id).Delete(&models.BlogPost{}).Error}
func(s *SchoolContentService) ListBlogs(schoolID uuid.UUID)([]models.BlogPost,error){var p []models.BlogPost;return p,s.db.Where("school_id=?",schoolID).Order("created_at DESC").Find(&p).Error}
func(s *SchoolContentService) PublicBlogs(schoolID uuid.UUID)([]models.BlogPost,error){var p []models.BlogPost;return p,s.db.Where("school_id=? AND status=?",schoolID,models.ContentPublished).Order("published_at DESC").Find(&p).Error}
func(s *SchoolContentService) PublicBlog(schoolID uuid.UUID,slug string)(models.BlogPost,error){var p models.BlogPost;if err:=s.db.Where("school_id=? AND lower(slug)=lower(?) AND status=?",schoolID,slug,models.ContentPublished).First(&p).Error;err!=nil{return p,ErrContentNotFound};return p,nil}

func(s *SchoolContentService) CreateAlbum(schoolID uuid.UUID,a *models.GalleryAlbum)(models.GalleryAlbum,error){
 a.ID=uuid.New();a.SchoolID=schoolID;a.Title=strings.TrimSpace(a.Title);a.Slug=slugify(a.Slug);if a.Slug==""{a.Slug=slugify(a.Title)};if a.Status==""{a.Status=models.ContentDraft};if !validStatus(a.Status){return *a,ErrContentInvalidStatus}
 var n int64;s.db.Model(&models.GalleryAlbum{}).Where("school_id=? AND lower(slug)=lower(?)",schoolID,a.Slug).Count(&n);if n>0{return *a,ErrContentSlugExists}
 if err:=s.db.Create(a).Error;err!=nil{return *a,err};return *a,nil
}
func(s *SchoolContentService) UpdateAlbum(schoolID,id uuid.UUID,updates map[string]interface{})(models.GalleryAlbum,error){var a models.GalleryAlbum;if err:=s.db.Where("school_id=? AND id=?",schoolID,id).First(&a).Error;err!=nil{return a,ErrContentNotFound};allowed:=map[string]bool{"title":true,"slug":true,"description":true,"cover_image_url":true,"status":true,"featured":true};safe:=map[string]interface{}{};for k,v:=range updates{if allowed[k]{safe[k]=v}};if v,ok:=safe["slug"].(string);ok{safe["slug"]=slugify(v);if safe["slug"]==""{safe["slug"]=a.Slug};var n int64;s.db.Model(&models.GalleryAlbum{}).Where("school_id=? AND lower(slug)=lower(?) AND id<>?",schoolID,safe["slug"],id).Count(&n);if n>0{return a,ErrContentSlugExists}};if v,ok:=safe["status"].(string);ok&&!validStatus(v){return a,ErrContentInvalidStatus};if err:=s.db.Model(&a).Updates(safe).Error;err!=nil{return a,err};return a,s.db.Where("school_id=? AND id=?",schoolID,id).First(&a).Error}
func(s *SchoolContentService) DeleteAlbum(schoolID,id uuid.UUID)error{return s.db.Transaction(func(tx *gorm.DB)error{if err:=tx.Where("school_id=? AND album_id=?",schoolID,id).Delete(&models.GalleryImage{}).Error;err!=nil{return err};return tx.Where("school_id=? AND id=?",schoolID,id).Delete(&models.GalleryAlbum{}).Error})}
func(s *SchoolContentService) ListAlbums(schoolID uuid.UUID)([]models.GalleryAlbum,error){var a []models.GalleryAlbum;return a,s.db.Where("school_id=?",schoolID).Order("created_at DESC").Find(&a).Error}
func(s *SchoolContentService) PublicAlbums(schoolID uuid.UUID)([]models.GalleryAlbum,error){var a []models.GalleryAlbum;return a,s.db.Where("school_id=? AND status=?",schoolID,models.ContentPublished).Order("created_at DESC").Find(&a).Error}
func(s *SchoolContentService) PublicAlbum(schoolID uuid.UUID,slug string)(models.GalleryAlbum,[]models.GalleryImage,error){var a models.GalleryAlbum;if err:=s.db.Where("school_id=? AND lower(slug)=lower(?) AND status=?",schoolID,slug,models.ContentPublished).First(&a).Error;err!=nil{return a,nil,ErrContentNotFound};var imgs []models.GalleryImage;if err:=s.db.Where("school_id=? AND album_id=?",schoolID,a.ID).Order("display_order ASC,created_at ASC").Find(&imgs).Error;err!=nil{return a,nil,err};return a,imgs,nil}
func(s *SchoolContentService) AddImage(schoolID,userID,albumID uuid.UUID,img *models.GalleryImage)(models.GalleryImage,error){var a models.GalleryAlbum;if err:=s.db.Where("school_id=? AND id=?",schoolID,albumID).First(&a).Error;err!=nil{return *img,ErrContentNotFound};img.ID=uuid.New();img.SchoolID=schoolID;img.AlbumID=albumID;img.UploadedBy=userID;if err:=s.db.Create(img).Error;err!=nil{return *img,err};return *img,nil}
func(s *SchoolContentService) DeleteImage(schoolID,id uuid.UUID)error{return s.db.Where("school_id=? AND id=?",schoolID,id).Delete(&models.GalleryImage{}).Error}
func(s *SchoolContentService) PublicHome(schoolID uuid.UUID)([]models.BlogPost,[]models.GalleryAlbum,error){blogs,err:=s.PublicBlogs(schoolID);if err!=nil{return nil,nil,err};albums,err:=s.PublicAlbums(schoolID);if err!=nil{return nil,nil,err};if len(blogs)>3{blogs=blogs[:3]};if len(albums)>4{albums=albums[:4]};return blogs,albums,nil}
