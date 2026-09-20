package service

import("testing";"github.com/onoja217/users-management-app/internal/authz";"github.com/onoja217/users-management-app/internal/models";"github.com/onoja217/users-management-app/internal/repository";"gorm.io/driver/sqlite";"gorm.io/gorm")
func TestGuardianRelationshipAccessAndSchoolIsolation(t *testing.T){
 db,err:=gorm.Open(sqlite.Open("file:guardian_test?mode=memory&cache=shared"),&gorm.Config{});if err!=nil{t.Fatal(err)}
 if err=db.AutoMigrate(&models.School{},&models.User{},&models.Student{},&models.AcademicSession{},&models.Term{},&models.SchoolClass{},&models.Section{},&models.StudentEnrollment{},&models.AttendanceRecord{},&models.Assessment{},&models.AssessmentResult{},&models.TeacherAssignment{},&models.Subject{},&models.TimetableEntry{},&models.Invoice{},&models.InvoiceLine{},&models.Payment{},&models.GuardianRelationship{});err!=nil{t.Fatal(err)}
 a:=models.School{Name:"School A",Code:"GA"};b:=models.School{Name:"School B",Code:"GB"};if err=db.Create(&a).Error;err!=nil{t.Fatal(err)};if err=db.Create(&b).Error;err!=nil{t.Fatal(err)}
 parentA:=models.User{Name:"Parent A",Email:"parent-a@test",Role:authz.RoleParent,Active:true,SchoolID:&a.ID};parentB:=models.User{Name:"Parent B",Email:"parent-b@test",Role:authz.RoleParent,Active:true,SchoolID:&b.ID};studentUser:=models.User{Name:"Student A",Email:"student-a@test",Role:authz.RoleStudent,Active:true,SchoolID:&a.ID}
 for _,u:=range []*models.User{&parentA,&parentB,&studentUser}{if err=db.Create(u).Error;err!=nil{t.Fatal(err)}}
 student:=models.Student{SchoolID:a.ID,UserID:studentUser.ID,AdmissionNumber:"ADM-A"};if err=db.Create(&student).Error;err!=nil{t.Fatal(err)}
 rel:=repository.NewGuardianRepository(db);if _,err=rel.Create(a.ID,models.GuardianRelationship{GuardianUserID:parentA.ID,StudentID:student.ID,Relationship:"mother",Active:true});err!=nil{t.Fatal(err)}
 svc:=NewGuardianService(rel,db)
 children,err:=svc.Children(a.ID,parentA.ID);if err!=nil||len(children)!=1{t.Fatalf("expected one child, err=%v len=%d",err,len(children))}
 if _,err=svc.Attendance(b.ID,parentB.ID,student.ID);err!=ErrGuardianForbidden{t.Fatalf("expected cross-school access to be forbidden, got %v",err)}
 if _,err=svc.CreateLink(b.ID,parentB.ID,student.ID,"father",false);err!=ErrGuardianInvalid{t.Fatalf("expected cross-school link rejection, got %v",err)}
 if _,err=svc.CreateLink(a.ID,parentA.ID,student.ID,"father",false);err!=ErrGuardianInvalid{t.Fatalf("expected duplicate link rejection, got %v",err)}
 if _,err=rel.Find(b.ID,parentA.ID,student.ID);!gorm.IsRecordNotFoundError(err){t.Fatalf("expected school-scoped repository lookup to miss, got %v",err)}
}
