package service

import (
 "testing"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/authz"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
)

func TestStudentPortalIsSchoolScoped(t *testing.T){
 db,err:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if err!=nil{t.Fatal(err)}
 if err=db.AutoMigrate(&models.School{},&models.User{},&models.Student{},&models.AcademicSession{},&models.Term{},&models.SchoolClass{},&models.Section{},&models.StudentEnrollment{},&models.AttendanceRecord{},&models.TimetableEntry{},&models.Invoice{},&models.InvoiceLine{},&models.Payment{});err!=nil{t.Fatal(err)}
 a:=models.School{Name:"School A",Code:"SPA"};b:=models.School{Name:"School B",Code:"SPB"};if err=db.Create(&a).Error;err!=nil{t.Fatal(err)};if err=db.Create(&b).Error;err!=nil{t.Fatal(err)}
 u1:=models.User{Name:"Student One",Email:"one-"+uuid.NewString()+"@test",Role:authz.RoleStudent,Active:true,SchoolID:&a.ID};u2:=models.User{Name:"Student Two",Email:"two-"+uuid.NewString()+"@test",Role:authz.RoleStudent,Active:true,SchoolID:&b.ID}
 if err=db.Create(&u1).Error;err!=nil{t.Fatal(err)};if err=db.Create(&u2).Error;err!=nil{t.Fatal(err)}
 s1:=models.Student{SchoolID:a.ID,UserID:u1.ID,AdmissionNumber:"ADM-"+uuid.NewString()};s2:=models.Student{SchoolID:b.ID,UserID:u2.ID,AdmissionNumber:"ADM-"+uuid.NewString()}
 if err=db.Create(&s1).Error;err!=nil{t.Fatal(err)};if err=db.Create(&s2).Error;err!=nil{t.Fatal(err)}
 sessionA:=models.AcademicSession{SchoolID:a.ID,Name:"2026/2027"};sessionB:=models.AcademicSession{SchoolID:b.ID,Name:"2026/2027"};if err=db.Create(&sessionA).Error;err!=nil{t.Fatal(err)};if err=db.Create(&sessionB).Error;err!=nil{t.Fatal(err)}
 classA:=models.SchoolClass{SchoolID:a.ID,Name:"JSS1",Level:1};classB:=models.SchoolClass{SchoolID:b.ID,Name:"JSS1",Level:1};if err=db.Create(&classA).Error;err!=nil{t.Fatal(err)};if err=db.Create(&classB).Error;err!=nil{t.Fatal(err)}
 secA:=models.Section{SchoolID:a.ID,ClassID:classA.ID,Name:"A"};secB:=models.Section{SchoolID:b.ID,ClassID:classB.ID,Name:"A"};if err=db.Create(&secA).Error;err!=nil{t.Fatal(err)};if err=db.Create(&secB).Error;err!=nil{t.Fatal(err)}
 e1:=models.StudentEnrollment{SchoolID:a.ID,StudentID:s1.ID,AcademicSessionID:sessionA.ID,ClassID:classA.ID,SectionID:secA.ID,Status:models.EnrollmentStatusActive};e2:=models.StudentEnrollment{SchoolID:b.ID,StudentID:s2.ID,AcademicSessionID:sessionB.ID,ClassID:classB.ID,SectionID:secB.ID,Status:models.EnrollmentStatusActive};if err=db.Create(&e1).Error;err!=nil{t.Fatal(err)};if err=db.Create(&e2).Error;err!=nil{t.Fatal(err)}
 svc:=NewStudentPortalService(db)
 p,err:=svc.Profile(a.ID,u1.ID);if err!=nil{t.Fatal(err)};if p.ID!=s1.ID||p.Name!="Student One"{t.Fatalf("wrong profile returned: %+v",p)}
 if _,err:=svc.Profile(b.ID,u1.ID);err!=ErrStudentPortalForbidden{t.Fatalf("expected cross-school profile rejection, got %v",err)}
 en,err:=svc.Enrollment(a.ID,u1.ID,sessionA.ID);if err!=nil{t.Fatal(err)};if en.ID!=e1.ID{t.Fatalf("wrong enrollment returned: %+v",en)}
 if _,err:=svc.Enrollment(b.ID,u1.ID,sessionA.ID);err!=ErrStudentPortalForbidden{t.Fatalf("expected cross-school enrollment rejection, got %v",err)}
 if _,err:=svc.Profile(a.ID,uuid.New());err!=ErrStudentPortalForbidden{t.Fatalf("expected forbidden for unknown user, got %v",err)}
}
