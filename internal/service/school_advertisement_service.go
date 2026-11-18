package service

import (
 "errors"
 "strings"
 "time"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/gorm"
)

var ErrAdvertisementNotFound=errors.New("advertisement not found")
var ErrAdvertisementInvalid=errors.New("invalid advertisement")

type SchoolAdvertisementService struct{db *gorm.DB}
func NewSchoolAdvertisementService(db *gorm.DB)*SchoolAdvertisementService{return &SchoolAdvertisementService{db:db}}

func(s *SchoolAdvertisementService) List(schoolID uuid.UUID)([]models.SchoolAdvertisement,error){
 var ads []models.SchoolAdvertisement
 return ads,s.db.Where("school_id=?",schoolID).Order("created_at DESC").Find(&ads).Error
}
func(s *SchoolAdvertisementService) Create(schoolID,actorID uuid.UUID,a *models.SchoolAdvertisement)(models.SchoolAdvertisement,error){
 a.ID=uuid.New();a.SchoolID=schoolID;a.CreatedBy=actorID;a.Title=strings.TrimSpace(a.Title);a.Description=strings.TrimSpace(a.Description);a.ButtonText=strings.TrimSpace(a.ButtonText);a.TargetURL=strings.TrimSpace(a.TargetURL)
 if a.Title==""||a.ImageURL!=""&&len(a.ImageURL)>1200{return *a,ErrAdvertisementInvalid}
 if a.Status==""{a.Status=models.ContentDraft}
 if !validStatus(a.Status){return *a,ErrAdvertisementInvalid}
 if a.StartsAt!=nil&&a.EndsAt!=nil&&a.EndsAt.Before(*a.StartsAt){return *a,ErrAdvertisementInvalid}
 if a.Status==models.ContentPublished&&!a.Featured{a.Featured=true}
 if err:=s.db.Create(a).Error;err!=nil{return *a,err}
 return *a,nil
}
func(s *SchoolAdvertisementService) Update(schoolID,id uuid.UUID,updates map[string]interface{})(models.SchoolAdvertisement,error){
 var a models.SchoolAdvertisement
 if err:=s.db.Where("school_id=? AND id=?",schoolID,id).First(&a).Error;err!=nil{return a,ErrAdvertisementNotFound}
 allowed:=map[string]bool{"title":true,"description":true,"image_url":true,"button_text":true,"target_url":true,"status":true,"featured":true,"starts_at":true,"ends_at":true}
 safe:=map[string]interface{}{}
 for k,v:=range updates{if allowed[k]{safe[k]=v}}
 if v,ok:=safe["status"].(string);ok&&!validStatus(v){return a,ErrAdvertisementInvalid}
 if v,ok:=safe["title"].(string);ok&&strings.TrimSpace(v)==""{return a,ErrAdvertisementInvalid}
 if err:=s.db.Model(&a).Updates(safe).Error;err!=nil{return a,err}
 if err:=s.db.Where("school_id=? AND id=?",schoolID,id).First(&a).Error;err!=nil{return a,err}
 if a.StartsAt!=nil&&a.EndsAt!=nil&&a.EndsAt.Before(*a.StartsAt){return a,ErrAdvertisementInvalid}
 return a,nil
}
func(s *SchoolAdvertisementService) Delete(schoolID,id uuid.UUID)error{
 r:=s.db.Where("school_id=? AND id=?",schoolID,id).Delete(&models.SchoolAdvertisement{})
 if r.Error!=nil{return r.Error};if r.RowsAffected==0{return ErrAdvertisementNotFound};return nil
}
func(s *SchoolAdvertisementService) Public(schoolID uuid.UUID)([]models.SchoolAdvertisement,error){
 now:=time.Now().UTC()
 var ads []models.SchoolAdvertisement
 err:=s.db.Where("school_id=? AND status=? AND image_url<>'' AND (starts_at IS NULL OR starts_at<=?) AND (ends_at IS NULL OR ends_at>=?)",schoolID,models.ContentPublished,now,now).Order("featured DESC,created_at DESC").Find(&ads).Error
 if err!=nil{return nil,err}
 if len(ads)>3{ads=ads[:3]}
 return ads,nil
}
