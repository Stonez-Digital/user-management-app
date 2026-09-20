package service

import (
    "errors"
    "testing"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/authz"
    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func coreAcademicIsolationDB(t *testing.T, modelsToMigrate ...interface{}) *gorm.DB {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file:core_academic_isolation_"+uuid.New().String()+"?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil {
        t.Fatal(err)
    }
    if err := db.AutoMigrate(modelsToMigrate...); err != nil {
        t.Fatal(err)
    }
    return db
}

func TestStudentServiceIsolatesSchools(t *testing.T) {
    db := coreAcademicIsolationDB(t, &models.School{}, &models.User{}, &models.Student{})
    schoolA := uuid.New()
    schoolB := uuid.New()

    userA := models.User{SchoolID: &schoolA, Name: "Student A", Email: "student-a@test", Role: authz.RoleStudent, Active: true}
    userB := models.User{SchoolID: &schoolB, Name: "Student B", Email: "student-b@test", Role: authz.RoleStudent, Active: true}
    if err := db.Create(&userA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&userB).Error; err != nil { t.Fatal(err) }

    repo := repository.NewStudentRepository(db)
    svc := NewStudentService(repo, db)

    studentA, err := svc.CreateStudent(schoolA, models.Student{UserID: userA.ID, AdmissionNumber: "ADM-001"})
    if err != nil { t.Fatal(err) }
    studentB, err := svc.CreateStudent(schoolB, models.Student{UserID: userB.ID, AdmissionNumber: "ADM-001"})
    if err != nil { t.Fatal(err) }

    studentsA, err := svc.GetStudents(schoolA)
    if err != nil { t.Fatal(err) }
    if len(studentsA) != 1 || studentsA[0].ID != studentA.ID { t.Fatalf("unexpected school A students: %+v", studentsA) }

    if _, err := svc.GetStudent(schoolA, studentB.ID); !errors.Is(err, ErrStudentNotFound) {
        t.Fatalf("expected cross-school student lookup to be hidden, got %v", err)
    }

    if err := svc.UpdateStudent(schoolA, models.Student{ID: studentB.ID, AdmissionNumber: "ADM-999"}); !errors.Is(err, ErrStudentNotFound) {
        t.Fatalf("expected cross-school update to be blocked, got %v", err)
    }
    if err := svc.DeleteStudent(schoolA, studentB.ID); !errors.Is(err, ErrStudentNotFound) {
        t.Fatalf("expected cross-school delete to be blocked, got %v", err)
    }

    if _, err := svc.CreateStudent(schoolA, models.Student{UserID: userA.ID, AdmissionNumber: "ADM-001"}); !errors.Is(err, ErrDuplicateAdmission) {
        t.Fatalf("expected school-scoped duplicate admission rejection, got %v", err)
    }
}

func TestClassAndSectionServiceIsolatesSchools(t *testing.T) {
    db := coreAcademicIsolationDB(t, &models.School{}, &models.SchoolClass{}, &models.Section{})
    schoolA := uuid.New()
    schoolB := uuid.New()
    svc := NewClassService(repository.NewClassRepository(db), repository.NewSectionRepository(db), db)

    classA, err := svc.CreateClass(schoolA, models.SchoolClass{Name: "JSS 1", Level: 1})
    if err != nil { t.Fatal(err) }
    classB, err := svc.CreateClass(schoolB, models.SchoolClass{Name: "JSS 1", Level: 1})
    if err != nil { t.Fatal(err) }

    sectionA, err := svc.CreateSection(schoolA, models.Section{ClassID: classA.ID, Name: "A"})
    if err != nil { t.Fatal(err) }
    sectionB, err := svc.CreateSection(schoolB, models.Section{ClassID: classB.ID, Name: "A"})
    if err != nil { t.Fatal(err) }

    if _, err := svc.GetClass(schoolA, classB.ID); !errors.Is(err, ErrClassNotFound) {
        t.Fatalf("expected cross-school class lookup to be hidden, got %v", err)
    }
    if _, err := svc.GetSection(schoolA, sectionB.ID); !errors.Is(err, ErrSectionNotFound) {
        t.Fatalf("expected cross-school section lookup to be hidden, got %v", err)
    }

    if err := svc.UpdateClass(schoolA, models.SchoolClass{ID: classB.ID, Name: "JSS 2", Level: 2}); !errors.Is(err, ErrClassNotFound) {
        t.Fatalf("expected cross-school class update to be blocked, got %v", err)
    }
    if err := svc.DeleteSection(schoolA, sectionB.ID); !errors.Is(err, ErrSectionNotFound) {
        t.Fatalf("expected cross-school section delete to be blocked, got %v", err)
    }

    if _, err := svc.CreateClass(schoolA, models.SchoolClass{Name: "JSS 1", Level: 1}); !errors.Is(err, ErrClassDuplicate) {
        t.Fatalf("expected school-scoped class duplicate rejection, got %v", err)
    }
    if _, err := svc.CreateSection(schoolA, models.Section{ClassID: classA.ID, Name: "A"}); !errors.Is(err, ErrSectionDuplicate) {
        t.Fatalf("expected school-scoped section duplicate rejection, got %v", err)
    }
}

func TestSubjectServiceIsolatesSchools(t *testing.T) {
    db := coreAcademicIsolationDB(t, &models.School{}, &models.Subject{})
    schoolA := uuid.New()
    schoolB := uuid.New()
    svc := NewSubjectService(repository.NewSubjectRepository(db), db)

    subjectA, err := svc.Create(schoolA, models.Subject{Code: "MTH101", Name: "Mathematics"})
    if err != nil { t.Fatal(err) }
    subjectB, err := svc.Create(schoolB, models.Subject{Code: "MTH101", Name: "Mathematics"})
    if err != nil { t.Fatal(err) }

    subjectsA, err := svc.List(schoolA)
    if err != nil { t.Fatal(err) }
    if len(subjectsA) != 1 || subjectsA[0].ID != subjectA.ID { t.Fatalf("unexpected school A subjects: %+v", subjectsA) }

    if _, err := svc.Get(schoolA, subjectB.ID); !errors.Is(err, ErrSubjectNotFound) {
        t.Fatalf("expected cross-school subject lookup to be hidden, got %v", err)
    }
    if err := svc.Update(schoolA, models.Subject{ID: subjectB.ID, Code: "SCI101", Name: "Science"}); !errors.Is(err, ErrSubjectNotFound) {
        t.Fatalf("expected cross-school subject update to be blocked, got %v", err)
    }
    if err := svc.Delete(schoolA, subjectB.ID); !errors.Is(err, ErrSubjectNotFound) {
        t.Fatalf("expected cross-school subject delete to be blocked, got %v", err)
    }

    if _, err := svc.Create(schoolA, models.Subject{Code: "MTH101", Name: "Mathematics"}); !errors.Is(err, ErrSubjectDuplicate) {
        t.Fatalf("expected school-scoped subject duplicate rejection, got %v", err)
    }
}

func TestEnrollmentServiceRejectsCrossSchoolRelationships(t *testing.T) {
    db := coreAcademicIsolationDB(t,
        &models.School{},
        &models.User{},
        &models.Student{},
        &models.AcademicSession{},
        &models.SchoolClass{},
        &models.Section{},
        &models.StudentEnrollment{},
    )
    schoolA := uuid.New()
    schoolB := uuid.New()

    userA := models.User{SchoolID: &schoolA, Name: "Student A", Email: "enrollment-a@test", Role: authz.RoleStudent, Active: true}
    userB := models.User{SchoolID: &schoolB, Name: "Student B", Email: "enrollment-b@test", Role: authz.RoleStudent, Active: true}
    if err := db.Create(&userA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&userB).Error; err != nil { t.Fatal(err) }

    studentA := models.Student{SchoolID: schoolA, UserID: userA.ID, AdmissionNumber: "ADM-A"}
    studentB := models.Student{SchoolID: schoolB, UserID: userB.ID, AdmissionNumber: "ADM-B"}
    if err := db.Create(&studentA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&studentB).Error; err != nil { t.Fatal(err) }

    sessionA := models.AcademicSession{SchoolID: schoolA, Name: "2026/2027"}
    sessionB := models.AcademicSession{SchoolID: schoolB, Name: "2026/2027"}
    classA := models.SchoolClass{SchoolID: schoolA, Name: "JSS 1", Level: 1}
    classB := models.SchoolClass{SchoolID: schoolB, Name: "JSS 1", Level: 1}
    if err := db.Create(&sessionA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&sessionB).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&classA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&classB).Error; err != nil { t.Fatal(err) }

    sectionA := models.Section{SchoolID: schoolA, ClassID: classA.ID, Name: "A"}
    sectionB := models.Section{SchoolID: schoolB, ClassID: classB.ID, Name: "A"}
    if err := db.Create(&sectionA).Error; err != nil { t.Fatal(err) }
    if err := db.Create(&sectionB).Error; err != nil { t.Fatal(err) }

    svc := NewEnrollmentService(repository.NewEnrollmentRepository(db), db)

    _, err := svc.Create(schoolA, models.StudentEnrollment{
        StudentID: studentB.ID,
        AcademicSessionID: sessionA.ID,
        ClassID: classA.ID,
        SectionID: sectionA.ID,
    })
    if !errors.Is(err, ErrEnrollmentStudentMissing) {
        t.Fatalf("expected cross-school student rejection, got %v", err)
    }

    _, err = svc.Create(schoolA, models.StudentEnrollment{
        StudentID: studentA.ID,
        AcademicSessionID: sessionB.ID,
        ClassID: classA.ID,
        SectionID: sectionA.ID,
    })
    if !errors.Is(err, ErrEnrollmentSessionMissing) {
        t.Fatalf("expected cross-school session rejection, got %v", err)
    }

    _, err = svc.Create(schoolA, models.StudentEnrollment{
        StudentID: studentA.ID,
        AcademicSessionID: sessionA.ID,
        ClassID: classB.ID,
        SectionID: sectionA.ID,
    })
    if !errors.Is(err, ErrEnrollmentClassMissing) {
        t.Fatalf("expected cross-school class rejection, got %v", err)
    }

    _, err = svc.Create(schoolA, models.StudentEnrollment{
        StudentID: studentA.ID,
        AcademicSessionID: sessionA.ID,
        ClassID: classA.ID,
        SectionID: sectionB.ID,
    })
    if !errors.Is(err, ErrEnrollmentSectionMissing) {
        t.Fatalf("expected cross-school section rejection, got %v", err)
    }

    enrollment, err := svc.Create(schoolA, models.StudentEnrollment{
        StudentID: studentA.ID,
        AcademicSessionID: sessionA.ID,
        ClassID: classA.ID,
        SectionID: sectionA.ID,
    })
    if err != nil { t.Fatal(err) }

    if err := svc.Update(schoolA, models.StudentEnrollment{
        ID: enrollment.ID,
        StudentID: studentB.ID,
        AcademicSessionID: sessionA.ID,
        ClassID: classA.ID,
        SectionID: sectionA.ID,
        Status: models.EnrollmentStatusActive,
    }); !errors.Is(err, ErrEnrollmentSchoolMismatch) {
        t.Fatalf("expected cross-school enrollment relationship update to be blocked, got %v", err)
    }

    if err := svc.Delete(schoolA, enrollment.ID); err != nil {
        t.Fatal(err)
    }
    if _, err := svc.Get(schoolA, enrollment.ID); !errors.Is(err, ErrEnrollmentNotFound) {
        t.Fatalf("expected deleted enrollment to be unavailable, got %v", err)
    }
}
