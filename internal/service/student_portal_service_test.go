package service

import (
 "testing"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/authz"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
)

func TestStudentPortalIsUserScoped(t *testing.T){
 db,err:=gorm.Open(sqlite.Open("file:student_portal_test?mode=memory&cache=shared"),&gorm.Config{});if err!=nil{t.Fatal(err)}
 if err=db.AutoMigrate(&models.User{},&models.Student{},&models.AcademicSession{},&models.Term{},&models.SchoolClass{},&models.Section{},&models.StudentEnrollment{},&models.AttendanceRecord{},&models.TimetableEntry{},&models.Invoice{},&models.InvoiceLine{},&models.Payment{});err!=nil{t.Fatal(err)}
 u1:=models.User{Name:"Student One",Email:"one-"+uuid.NewString()+"@test",Role:authz.RoleStudent,Active:true};u2:=models.User{Name:"Student Two",Email:"two-"+uuid.NewString()+"@test",Role:authz.RoleStudent,Active:true}
 if err=db.Create(&u1).Error;err!=nil{t.Fatal(err)};if err=db.Create(&u2).Error;err!=nil{t.Fatal(err)}
 s1:=models.Student{UserID:u1.ID,AdmissionNumber:"ADM-"+uuid.NewString()};s2:=models.Student{UserID:u2.ID,AdmissionNumber:"ADM-"+uuid.NewString()}
 if err=db.Create(&s1).Error;err!=nil{t.Fatal(err)};if err=db.Create(&s2).Error;err!=nil{t.Fatal(err)}
 session:=models.AcademicSession{Name:"2026/2027"};if err=db.Create(&session).Error;err!=nil{t.Fatal(err)}
 class:=models.SchoolClass{Name:"JSS1",Level:1};if err=db.Create(&class).Error;err!=nil{t.Fatal(err)}
 sec:=models.Section{ClassID:class.ID,Name:"A"};if err=db.Create(&sec).Error;err!=nil{t.Fatal(err)}
 e1:=models.StudentEnrollment{StudentID:s1.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:sec.ID,Status:models.EnrollmentStatusActive};e2:=models.StudentEnrollment{StudentID:s2.ID,AcademicSessionID:session.ID,ClassID:class.ID,SectionID:sec.ID,Status:models.EnrollmentStatusActive}
 if err=db.Create(&e1).Error;err!=nil{t.Fatal(err)};if err=db.Create(&e2).Error;err!=nil{t.Fatal(err)}
 svc:=NewStudentPortalService(db)
 p,err:=svc.Profile(u1.ID);if err!=nil{t.Fatal(err)};if p.ID!=s1.ID||p.Name!="Student One"{t.Fatalf("wrong profile returned: %+v",p)}
 en,err:=svc.Enrollment(u1.ID);if err!=nil{t.Fatal(err)};if en.ID!=e1.ID{t.Fatalf("wrong enrollment returned: %+v",en)}
 if _,err:=svc.Profile(uuid.New());err!=ErrStudentPortalForbidden{t.Fatalf("expected forbidden for unknown user, got %v",err)}
 a:=models.AttendanceRecord{EnrollmentID:e1.ID,TermID:uuid.New(),Date:session.StartDate,Status:models.AttendancePresent};_ = a
 if _,err:=svc.Invoices(u2.ID);err!=nil{t.Fatal(err)}
}
