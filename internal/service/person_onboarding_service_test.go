package service

import(
 "testing"
 "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
)

func onboardingTestDB(t *testing.T)*gorm.DB{
 t.Helper()
 db,e:=gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),&gorm.Config{});if e!=nil{t.Fatal(e)}
 if e=db.AutoMigrate(&models.School{},&models.User{},&models.Student{},&models.TeacherProfile{},&models.ParentProfile{},&models.GuardianRelationship{});e!=nil{t.Fatal(e)}
 return db
}

func TestPersonOnboardingCreatesStudentAtomically(t *testing.T){
 db:=onboardingTestDB(t);school:=models.School{Name:"School A",Code:"ONB-A",Status:models.SchoolStatusActive};if e:=db.Create(&school).Error;e!=nil{t.Fatal(e)}
 svc:=NewPersonOnboardingService(db)
 out,e:=svc.Onboard(school.ID,PersonOnboardingRequest{Name:"Student One",Email:"student-one@example.com",Password:"password123",Role:"student",AdmissionNumber:"BED-001"});if e!=nil{t.Fatal(e)}
 if out.User.ID==uuid.Nil||out.Student==nil||out.Student.UserID!=out.User.ID{t.Fatal("expected user and student profile to be created together")}
 var users,students int64;db.Model(&models.User{}).Where("school_id = ?",school.ID).Count(&users);db.Model(&models.Student{}).Where("school_id = ?",school.ID).Count(&students);if users!=1||students!=1{t.Fatalf("expected one user and student, got %d/%d",users,students)}
}

func TestPersonOnboardingParentRollbackLeavesNoOrphanUser(t *testing.T){
 db:=onboardingTestDB(t);school:=models.School{Name:"School A",Code:"ONB-B",Status:models.SchoolStatusActive};if e:=db.Create(&school).Error;e!=nil{t.Fatal(e)}
 svc:=NewPersonOnboardingService(db)
 _,e:=svc.Onboard(school.ID,PersonOnboardingRequest{Name:"Parent One",Email:"parent-one@example.com",Password:"password123",Role:"parent",StudentID:uuid.New(),Relationship:"mother",Primary:true});if e!=ErrOnboardingStudentNotFound{t.Fatalf("expected missing student error, got %v",e)}
 var users int64;db.Model(&models.User{}).Where("school_id = ?",school.ID).Count(&users);if users!=0{t.Fatalf("rollback failed: expected no user, got %d",users)}
}

func TestPersonOnboardingRejectsCrossSchoolParentLink(t *testing.T){
 db:=onboardingTestDB(t);a:=models.School{Name:"School A",Code:"ONB-C",Status:models.SchoolStatusActive};b:=models.School{Name:"School B",Code:"ONB-D",Status:models.SchoolStatusActive};if e:=db.Create(&a).Error;e!=nil{t.Fatal(e)};if e:=db.Create(&b).Error;e!=nil{t.Fatal(e)}
 studentUser:=models.User{Name:"Student",Email:"cross-student@example.com",Role:"student",Active:true,SchoolID:&b.ID};if e:=db.Create(&studentUser).Error;e!=nil{t.Fatal(e)}
 student:=models.Student{SchoolID:b.ID,UserID:studentUser.ID,AdmissionNumber:"B-001"};if e:=db.Create(&student).Error;e!=nil{t.Fatal(e)}
 svc:=NewPersonOnboardingService(db)
 _,e:=svc.Onboard(a.ID,PersonOnboardingRequest{Name:"Parent",Email:"cross-parent@example.com",Password:"password123",Role:"parent",StudentID:student.ID,Relationship:"parent",Primary:true});if e!=ErrOnboardingStudentNotFound{t.Fatalf("expected cross-school student rejection, got %v",e)}
 var users int64;db.Model(&models.User{}).Where("school_id = ?",a.ID).Count(&users);if users!=0{t.Fatalf("cross-school rollback failed: got %d users",users)}
}

func TestPersonOnboardingCreatesTeacherAndParentLinkAtomically(t *testing.T){
 db:=onboardingTestDB(t);school:=models.School{Name:"School A",Code:"ONB-E",Status:models.SchoolStatusActive};if e:=db.Create(&school).Error;e!=nil{t.Fatal(e)}
 studentUser:=models.User{Name:"Existing Student",Email:"existing-student@example.com",Role:"student",Active:true,SchoolID:&school.ID};if e:=db.Create(&studentUser).Error;e!=nil{t.Fatal(e)}
 student:=models.Student{SchoolID:school.ID,UserID:studentUser.ID,AdmissionNumber:"E-001"};if e:=db.Create(&student).Error;e!=nil{t.Fatal(e)}
 svc:=NewPersonOnboardingService(db)
 teacher,e:=svc.Onboard(school.ID,PersonOnboardingRequest{Name:"Teacher One",Email:"teacher-one@example.com",Password:"password123",Role:"teacher",StaffID:"T-001",Phone:"08000000000",Gender:"female",Department:"Science",Designation:"Teacher",Subjects:"Mathematics",Classes:"JSS1"});if e!=nil||teacher.User.Role!="teacher"{t.Fatalf("teacher onboarding: %v",e)};var tp models.TeacherProfile;if e=db.Where("user_id = ?",teacher.User.ID).First(&tp).Error;e!=nil{t.Fatalf("teacher profile missing: %v",e)};if tp.StaffID!="T-001"||tp.SchoolID!=school.ID{t.Fatal("teacher profile is not school-scoped")}
 parent,e:=svc.Onboard(school.ID,PersonOnboardingRequest{Name:"Parent One",Email:"parent-one@example.com",Password:"password123",Role:"parent",ParentIdentifier:"PARENT-001",Phone:"08000000001",Address:"Otukpo",Occupation:"Trader",StudentID:student.ID,Relationship:"mother",Primary:true});if e!=nil{t.Fatal(e)}
 if parent.GuardianLink==nil||parent.GuardianLink.StudentID!=student.ID||parent.GuardianLink.GuardianUserID!=parent.User.ID{t.Fatal("expected parent and guardian link to be created together")};var pp models.ParentProfile;if e=db.Where("user_id = ?",parent.User.ID).First(&pp).Error;e!=nil{t.Fatalf("parent profile missing: %v",e)};if pp.ParentIdentifier!="parent-001"||pp.SchoolID!=school.ID{t.Fatal("parent profile is not school-scoped")}
 var links int64;db.Model(&models.GuardianRelationship{}).Where("school_id = ?",school.ID).Count(&links);if links!=1{t.Fatalf("expected one guardian link, got %d",links)}
}


func TestPersonOnboardingRejectsTeacherWithoutStaffID(t *testing.T) {
 db:=onboardingTestDB(t);school:=models.School{Name:"School A",Code:"ONB-F",Status:models.SchoolStatusActive};if e:=db.Create(&school).Error;e!=nil{t.Fatal(e)}
 svc:=NewPersonOnboardingService(db)
 _,e:=svc.Onboard(school.ID,PersonOnboardingRequest{Name:"Teacher",Email:"teacher-f@example.com",Password:"password123",Role:"teacher"})
 if e!=ErrOnboardingTeacherStaffIDRequired{t.Fatalf("expected staff ID error, got %v",e)}
 var n int64;db.Model(&models.User{}).Where("school_id = ?",school.ID).Count(&n);if n!=0{t.Fatalf("expected rollback, got %d users",n)}
}

func TestPersonOnboardingRejectsDuplicateTeacherStaffID(t *testing.T) {
 db:=onboardingTestDB(t);school:=models.School{Name:"School A",Code:"ONB-G",Status:models.SchoolStatusActive};if e:=db.Create(&school).Error;e!=nil{t.Fatal(e)}
 svc:=NewPersonOnboardingService(db)
 _,e:=svc.Onboard(school.ID,PersonOnboardingRequest{Name:"Teacher One",Email:"teacher-g1@example.com",Password:"password123",Role:"teacher",StaffID:"T-007"});if e!=nil{t.Fatal(e)}
 _,e=svc.Onboard(school.ID,PersonOnboardingRequest{Name:"Teacher Two",Email:"teacher-g2@example.com",Password:"password123",Role:"teacher",StaffID:"T-007"});if e!=ErrOnboardingTeacherProfileExists{t.Fatalf("expected duplicate staff error, got %v",e)}
}
