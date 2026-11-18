package service
import("testing";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/authz";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/driver/sqlite";"gorm.io/gorm")
func TestNotificationPublishAndSchoolIsolation(t *testing.T){db,e:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if e!=nil{t.Fatal(e)};if e=db.AutoMigrate(&models.School{},&models.User{},&models.Announcement{},&models.Notification{});e!=nil{t.Fatal(e)};a:=models.School{Name:"School A",Code:"NA"};b:=models.School{Name:"School B",Code:"NB"};if e=db.Create(&a).Error;e!=nil{t.Fatal(e)};if e=db.Create(&b).Error;e!=nil{t.Fatal(e)};pa:=models.User{Name:"Parent A",Email:"pa-"+uuid.NewString()+"@test",Role:"parent",Active:true,SchoolID:&a.ID};pb:=models.User{Name:"Parent B",Email:"pb-"+uuid.NewString()+"@test",Role:"parent",Active:true,SchoolID:&b.ID};if e=db.Create(&pa).Error;e!=nil{t.Fatal(e)};if e=db.Create(&pb).Error;e!=nil{t.Fatal(e)};r:=repository.NewNotificationRepository(db);s:=NewNotificationService(r,db);x,e:=s.CreateAnnouncement(b.ID,models.Announcement{CreatedBy:pb.ID,Title:"Fees",Body:"Payment due",Category:"fees",Audience:models.AudienceParents});if e!=nil{t.Fatal(e)};if _,e=s.CreateAnnouncement(a.ID,models.Announcement{CreatedBy:pb.ID,Title:"Cross",Body:"Denied",Category:"fees",Audience:models.AudienceParents});e!=ErrNotificationInvalid{t.Fatalf("expected cross-school creator rejection, got %v",e)};if v,e:=s.ListAnnouncements(a.ID);e!=nil||len(v)!=0{t.Fatalf("school A must not see B announcements: %d %v",len(v),e)};n,e:=s.Publish(b.ID,x.ID);if e!=nil||n!=1{t.Fatalf("publish: %d %v",n,e)};if v,e:=s.List(a.ID,pa.ID);e!=nil||len(v)!=0{t.Fatalf("school A user must not receive B notification: %d %v",len(v),e)};if v,e:=s.List(b.ID,pb.ID);e!=nil||len(v)!=1{t.Fatalf("school B parent should receive notification: %d %v",len(v),e)};if e=s.MarkRead(a.ID,pa.ID,x.ID);e!=nil{t.Fatal(e)}}

func TestNotificationAudienceGroups(t *testing.T){db,e:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if e!=nil{t.Fatal(e)};if e=db.AutoMigrate(&models.School{},&models.User{},&models.Announcement{},&models.Notification{});e!=nil{t.Fatal(e)};school:=models.School{Name:"School",Code:"N"};if e=db.Create(&school).Error;e!=nil{t.Fatal(e)};users:=[]models.User{{Name:"Admin",Email:"admin-"+uuid.NewString()+"@test",Role:"school_admin",Active:true,SchoolID:&school.ID},{Name:"Teacher",Email:"teacher-"+uuid.NewString()+"@test",Role:"teacher",Active:true,SchoolID:&school.ID},{Name:"Student",Email:"student-"+uuid.NewString()+"@test",Role:"student",Active:true,SchoolID:&school.ID},{Name:"Parent",Email:"parent-"+uuid.NewString()+"@test",Role:"parent",Active:true,SchoolID:&school.ID}};for i:=range users{if e=db.Create(&users[i]).Error;e!=nil{t.Fatal(e)}};r:=repository.NewNotificationRepository(db);s:=NewNotificationService(r,db);groups:=[]struct{audience,role string}{{models.AudienceTeachers,authz.RoleTeacher},{models.AudienceStudents,authz.RoleStudent},{models.AudienceParents,authz.RoleParent}};for _,g:=range groups{a,e:=s.CreateAnnouncement(school.ID,models.Announcement{CreatedBy:users[0].ID,Title:"Test",Body:"Message",Category:"general",Audience:g.audience});if e!=nil{t.Fatal(e)};n,e:=s.Publish(school.ID,a.ID);if e!=nil||n!=1{t.Fatalf("%s publish: %d %v",g.audience,n,e)};var recipient models.User;for i:=range users{if users[i].Role==g.role{recipient=users[i]}};v,e:=s.List(school.ID,recipient.ID);if e!=nil||len(v)!=1{t.Fatalf("%s recipient should receive exactly one notification: %d %v",g.audience,len(v),e)}}}

func TestNotificationPublishUsesRequestTransaction(t *testing.T) {
    db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.School{}, &models.User{}, &models.Announcement{}, &models.Notification{}); err != nil { t.Fatal(err) }
    school := models.School{Name: "Transactional School", Code: "TX-NOTIFY"}
    if err := db.Create(&school).Error; err != nil { t.Fatal(err) }
    parent := models.User{Name: "Parent", Email: "tx-parent-"+uuid.NewString()+"@test", Role: authz.RoleParent, Active: true, SchoolID: &school.ID}
    if err := db.Create(&parent).Error; err != nil { t.Fatal(err) }
    base := NewNotificationService(repository.NewNotificationRepository(db), db)
    announcement, err := base.CreateAnnouncement(school.ID, models.Announcement{CreatedBy: parent.ID, Title: "Term notice", Body: "Notice", Category: "general", Audience: models.AudienceParents})
    if err != nil { t.Fatal(err) }

    tx := db.Begin()
    if tx.Error != nil { t.Fatal(tx.Error) }
    scoped := base.WithDB(tx)
    if count, err := scoped.Publish(school.ID, announcement.ID); err != nil || count != 1 {
        _ = tx.Rollback().Error
        t.Fatalf("expected one transactional notification, got %d: %v", count, err)
    }
    if err := tx.Rollback().Error; err != nil { t.Fatal(err) }

    var notificationCount int64
    if err := db.Model(&models.Notification{}).Where("school_id = ? AND user_id = ?", school.ID, parent.ID).Count(&notificationCount).Error; err != nil { t.Fatal(err) }
    if notificationCount != 0 { t.Fatalf("notification escaped rolled-back request transaction: %d", notificationCount) }
    var stored models.Announcement
    if err := db.First(&stored, "id = ?", announcement.ID).Error; err != nil { t.Fatal(err) }
    if stored.Published { t.Fatal("announcement publication escaped rolled-back request transaction") }
}
