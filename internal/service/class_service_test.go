package service

import (
 "testing"
    "github.com/google/uuid"
 "github.com/onoja217/users-management-app/internal/models"
 "github.com/onoja217/users-management-app/internal/repository"
 "gorm.io/driver/sqlite"
 "gorm.io/gorm"
)

func classTestService(t *testing.T)*ClassService{
 t.Helper()
 db,e:=gorm.Open(sqlite.Open("file:class_test?mode=memory&cache=shared"),&gorm.Config{})
 if e!=nil{t.Fatal(e)}
 if e=db.AutoMigrate(&models.SchoolClass{},&models.Section{});e!=nil{t.Fatal(e)}
 return NewClassService(repository.NewClassRepository(db),repository.NewSectionRepository(db),db)
}
func TestClassAndSectionLifecycle(t *testing.T){
 s:=classTestService(t)
 c,e:=s.CreateClass(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.SchoolClass{SchoolID:uuid.MustParse("00000000-0000-0000-0000-000000000001"),Name:"JSS 1",Level:1});if e!=nil{t.Fatal(e)}
 sec,e:=s.CreateSection(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.Section{SchoolID:uuid.MustParse("00000000-0000-0000-0000-000000000001"),ClassID:c.ID,Name:"A"});if e!=nil{t.Fatal(e)}
 if sec.ClassID!=c.ID{t.Fatal("section not linked to class")}
 got,e:=s.GetClass(uuid.MustParse("00000000-0000-0000-0000-000000000001"),c.ID);if e!=nil{t.Fatal(e)}
 if len(got.Sections)!=1{t.Fatalf("expected 1 section, got %d",len(got.Sections))}
}
func TestDuplicateSectionRejected(t *testing.T){
 s:=classTestService(t);c,e:=s.CreateClass(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.SchoolClass{Name:"JSS 2",Level:2});if e!=nil{t.Fatal(e)}
 if _,e=s.CreateSection(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.Section{ClassID:c.ID,Name:"A"});e!=nil{t.Fatal(e)}
 if _,e=s.CreateSection(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.Section{ClassID:c.ID,Name:"A"});e!=ErrSectionDuplicate{t.Fatalf("expected duplicate error, got %v",e)}
}
func TestDuplicateClassRejected(t *testing.T){
 s:=classTestService(t);_,e:=s.CreateClass(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.SchoolClass{Name:"SS 1",Level:3});if e!=nil{t.Fatal(e)}
 if _,e=s.CreateClass(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.SchoolClass{Name:"SS 1",Level:3});e!=ErrClassDuplicate{t.Fatalf("expected duplicate error, got %v",e)}
 if _,e=s.CreateClass(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.SchoolClass{Name:" ss 1 ",Level:3});e!=ErrClassDuplicate{t.Fatalf("expected case-insensitive duplicate error, got %v",e)}
 if _,e=s.CreateClass(uuid.MustParse("00000000-0000-0000-0000-000000000002"),models.SchoolClass{Name:"SS 1",Level:3});e!=nil{t.Fatalf("same class name should be allowed in another school: %v",e)}
}
