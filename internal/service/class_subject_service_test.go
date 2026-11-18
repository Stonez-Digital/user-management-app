package service

import (
    "fmt"
    "testing"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func classSubjectTestDB(t *testing.T) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err = db.AutoMigrate(
        &models.School{}, &models.AcademicSession{}, &models.SchoolClass{},
        &models.Subject{}, &models.ClassSubject{},
    ); err != nil { t.Fatal(err) }
    return db
}

func TestClassSubjectRejectsCrossSchoolRelationship(t *testing.T) {
    db := classSubjectTestDB(t)
    schoolA := models.School{ID:uuid.New(),Name:"A",Code:"A",Status:models.SchoolStatusActive}
    schoolB := models.School{ID:uuid.New(),Name:"B",Code:"B",Status:models.SchoolStatusActive}
    if err:=db.Create(&schoolA).Error;err!=nil{t.Fatal(err)}
    if err:=db.Create(&schoolB).Error;err!=nil{t.Fatal(err)}
    sessionB:=models.AcademicSession{SchoolID:schoolB.ID,Name:"2026/2027"}
    classB:=models.SchoolClass{SchoolID:schoolB.ID,Name:"JSS 1",Level:1}
    subjectB:=models.Subject{SchoolID:schoolB.ID,Code:"MTH",Name:"Mathematics",Active:true}
    if err:=db.Create(&sessionB).Error;err!=nil{t.Fatal(err)}
    if err:=db.Create(&classB).Error;err!=nil{t.Fatal(err)}
    if err:=db.Create(&subjectB).Error;err!=nil{t.Fatal(err)}
    svc:=NewClassSubjectService(repository.NewClassSubjectRepository(db),db)
    _,err:=svc.Create(schoolA.ID,models.ClassSubject{
        AcademicSessionID:sessionB.ID,ClassID:classB.ID,SubjectID:subjectB.ID,
        CurriculumVersion:"NERDC-REVISED",Category:models.SubjectCategoryCore,Required:true,Active:true,
    })
    if err!=ErrClassSubjectInvalid{t.Fatalf("expected cross-school rejection, got %v",err)}
}

func TestClassSubjectDuplicateRejectedWithinSchoolSession(t *testing.T) {
    db := classSubjectTestDB(t)
    school:=models.School{ID:uuid.New(),Name:"A",Code:"A",Status:models.SchoolStatusActive}
    if err:=db.Create(&school).Error;err!=nil{t.Fatal(err)}
    session:=models.AcademicSession{SchoolID:school.ID,Name:"2026/2027"}
    class:=models.SchoolClass{SchoolID:school.ID,Name:"JSS 1",Level:1}
    subject:=models.Subject{SchoolID:school.ID,Code:"MTH",Name:"Mathematics",Active:true}
    if err:=db.Create(&session).Error;err!=nil{t.Fatal(err)}
    if err:=db.Create(&class).Error;err!=nil{t.Fatal(err)}
    if err:=db.Create(&subject).Error;err!=nil{t.Fatal(err)}
    svc:=NewClassSubjectService(repository.NewClassSubjectRepository(db),db)
    v:=models.ClassSubject{AcademicSessionID:session.ID,ClassID:class.ID,SubjectID:subject.ID,CurriculumVersion:"NERDC-REVISED",Category:models.SubjectCategoryCore,Required:true,Active:true}
    if _,err:=svc.Create(school.ID,v);err!=nil{t.Fatal(err)}
    if _,err:=svc.Create(school.ID,v);err!=ErrClassSubjectDuplicate{t.Fatalf("expected duplicate rejection, got %v",err)}
}
