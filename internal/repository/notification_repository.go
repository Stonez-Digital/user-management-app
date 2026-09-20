package repository
import("github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"gorm.io/gorm")
type NotificationRepository interface{CreateAnnouncement(uuid.UUID,models.Announcement)(models.Announcement,error);ListAnnouncements(uuid.UUID)([]models.Announcement,error);GetAnnouncement(uuid.UUID,uuid.UUID)(models.Announcement,error);CreateNotification(uuid.UUID,models.Notification)(models.Notification,error);ListForUser(uuid.UUID,uuid.UUID)([]models.Notification,error);MarkRead(uuid.UUID,uuid.UUID,uuid.UUID)error}
type notificationRepo struct{db *gorm.DB}
func NewNotificationRepository(db *gorm.DB)NotificationRepository{return &notificationRepo{db}}
func(r *notificationRepo)CreateAnnouncement(schoolID uuid.UUID,a models.Announcement)(models.Announcement,error){a.SchoolID=schoolID;return a,r.db.Create(&a).Error}
func(r *notificationRepo)ListAnnouncements(schoolID uuid.UUID)([]models.Announcement,error){var a []models.Announcement;e:=r.db.Where("school_id=?",schoolID).Order("created_at DESC").Find(&a).Error;return a,e}
func(r *notificationRepo)GetAnnouncement(schoolID,id uuid.UUID)(models.Announcement,error){var a models.Announcement;e:=r.db.Where("school_id=? AND id=?",schoolID,id).First(&a).Error;return a,e}
func(r *notificationRepo)CreateNotification(schoolID uuid.UUID,n models.Notification)(models.Notification,error){n.SchoolID=schoolID;return n,r.db.Create(&n).Error}
func(r *notificationRepo)ListForUser(schoolID,userID uuid.UUID)([]models.Notification,error){var n []models.Notification;e:=r.db.Where("school_id=? AND user_id=?",schoolID,userID).Order("created_at DESC").Find(&n).Error;return n,e}
func(r *notificationRepo)MarkRead(schoolID,userID,nid uuid.UUID)error{return r.db.Model(&models.Notification{}).Where("school_id=? AND id=? AND user_id=?",schoolID,nid,userID).Updates(map[string]interface{}{"read":true}).Error}
