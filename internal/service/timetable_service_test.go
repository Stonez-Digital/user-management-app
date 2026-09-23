package service

import("errors";"testing";"time";"github.com/google/uuid";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/driver/sqlite";"gorm.io/gorm")
func timetableDB(t *testing.T)*gorm.DB{db,e:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if e!=nil{t.Fatal(e)};if e=db.AutoMigrate(&models.School{},&models.User{},&models.AcademicSession{},&models.Term{},&models.SchoolClass{},&models.Section{},&models.Subject{},&models.TeacherAssignment{},&models.TimetableEntry{});e!=nil{t.Fatal(e)};return db}
func TestTimetableConflictValidationAndSchoolIsolation(t *testing.T){
 db:=timetableDB(t);a:=models.School{Name:"School A",Code:"TA"};b:=models.School{Name:"School B",Code:"TB"};for _,s:=range []*models.School{&a,&b}{if e:=db.Create(s).Error;e!=nil{t.Fatal(e)}}
 teacher:=models.User{Name:"Teacher",Email:"teacher-"+uuid.NewString()+"@example.com",Role:"teacher",Active:true,SchoolID:&a.ID};if e:=db.Create(&teacher).Error;e!=nil{t.Fatal(e)}
 sess:=models.AcademicSession{SchoolID:a.ID,Name:"2026/2027",StartDate:time.Date(2026,9,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,7,31,0,0,0,0,time.UTC)};if e:=db.Create(&sess).Error;e!=nil{t.Fatal(e)}
 term:=models.Term{SchoolID:a.ID,AcademicSessionID:sess.ID,Name:models.TermFirst,StartDate:sess.StartDate,EndDate:time.Date(2026,12,15,0,0,0,0,time.UTC)};if e:=db.Create(&term).Error;e!=nil{t.Fatal(e)}
 class:=models.SchoolClass{SchoolID:a.ID,Name:"JSS "+uuid.NewString()[:5],Level:1};if e:=db.Create(&class).Error;e!=nil{t.Fatal(e)};section:=models.Section{SchoolID:a.ID,ClassID:class.ID,Name:"A"};if e:=db.Create(&section).Error;e!=nil{t.Fatal(e)}
 subject:=models.Subject{SchoolID:a.ID,Code:"MAT-"+uuid.NewString()[:5],Name:"Mathematics "+uuid.NewString()[:5]};if e:=db.Create(&subject).Error;e!=nil{t.Fatal(e)}
 assignment:=models.TeacherAssignment{SchoolID:a.ID,TeacherID:teacher.ID,SubjectID:subject.ID,AcademicSessionID:sess.ID,TermID:term.ID,ClassID:class.ID,SectionID:&section.ID,Active:true};if e:=db.Create(&assignment).Error;e!=nil{t.Fatal(e)}
 svc:=NewTimetableService(repository.NewTimetableRepository(db),db)
 first,e:=svc.Create(a.ID,models.TimetableEntry{AcademicSessionID:sess.ID,TermID:term.ID,TeacherAssignmentID:assignment.ID,ClassID:class.ID,SectionID:&section.ID,DayOfWeek:models.TimetableMonday,StartTime:"08:00",EndTime:"09:00",Active:true});if e!=nil{t.Fatal(e)};if first.SchoolID!=a.ID{t.Fatal("create must force school id")}
 if _,e=svc.Create(a.ID,models.TimetableEntry{AcademicSessionID:sess.ID,TermID:term.ID,TeacherAssignmentID:assignment.ID,ClassID:class.ID,SectionID:&section.ID,DayOfWeek:models.TimetableMonday,StartTime:"08:30",EndTime:"09:30",Active:true});e!=ErrTimetableConflict{t.Fatalf("expected conflict, got %v",e)}
 if _,e=svc.Get(b.ID,first.ID);!errors.Is(e,ErrTimetableNotFound){t.Fatalf("expected cross-school get to fail, got %v",e)}
 if e=svc.Delete(b.ID,first.ID);!errors.Is(e,ErrTimetableNotFound){t.Fatalf("expected cross-school delete to fail, got %v",e)}
 if e=svc.Update(b.ID,models.TimetableEntry{ID:first.ID,AcademicSessionID:sess.ID,TermID:term.ID,TeacherAssignmentID:assignment.ID,ClassID:class.ID,SectionID:&section.ID,DayOfWeek:models.TimetableMonday,StartTime:"10:00",EndTime:"11:00",Active:true});!errors.Is(e,ErrTimetableNotFound){t.Fatalf("expected cross-school update to fail, got %v",e)}
 list,e:=svc.List(b.ID, sess.ID, term.ID);if e!=nil||len(list)!=0{t.Fatalf("expected empty school B list, len=%d err=%v",len(list),e)}
}
