package repository
import("github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"gorm.io/gorm")
type NotificationRepository interface{CreateAnnouncement(models.Announcement)(models.Announcement,error);ListAnnouncements()([]models.Announcement,error);CreateNotification(models.Notification)(models.Notification,error);ListForUser(uuid.UUID)([]models.Notification,error);MarkRead(uuid.UUID,uuid.UUID)error}
type notificationRepo struct{db *gorm.DB}
func NewNotificationRepository(db *gorm.DB)NotificationRepository{return &notificationRepo{db}}
func(r *notificationRepo)CreateAnnouncement(a models.Announcement)(models.Announcement,error){return a,r.db.Create(&a).Error}
func(r *notificationRepo)ListAnnouncements()([]models.Announcement,error){var a []models.Announcement;e:=r.db.Order("created_at DESC").Find(&a).Error;return a,e}
func(r *notificationRepo)CreateNotification(n models.Notification)(models.Notification,error){return n,r.db.Create(&n).Error}
func(r *notificationRepo)ListForUser(id uuid.UUID)([]models.Notification,error){var n []models.Notification;e:=r.db.Where("user_id=?",id).Order("created_at DESC").Find(&n).Error;return n,e}
func(r *notificationRepo)MarkRead(uid,nid uuid.UUID)error{return r.db.Model(&models.Notification{}).Where("id=? AND user_id=?",nid,uid).Updates(map[string]interface{}{"read":true}).Error}
