package service

import (
    "testing"
    "time"

    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func academicTestService(t *testing.T) *AcademicService {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file:academic_test?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.AcademicSession{}, &models.Term{}); err != nil { t.Fatal(err) }
    return NewAcademicService(repository.NewAcademicSessionRepository(db), repository.NewTermRepository(db), db)
}

func TestCreateSessionAndTerm(t *testing.T) {
    s := academicTestService(t)
    start := time.Date(2026,9,1,0,0,0,0,time.UTC)
    end := time.Date(2027,7,31,0,0,0,0,time.UTC)
    session, err := s.CreateSession(models.AcademicSession{Name:"2026/2027",StartDate:start,EndDate:end})
    if err != nil { t.Fatal(err) }
    if session.Status != models.AcademicStatusPlanned { t.Fatalf("expected planned, got %s",session.Status) }

    term, err := s.CreateTerm(models.Term{AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:start,EndDate:time.Date(2026,12,20,0,0,0,0,time.UTC)})
    if err != nil { t.Fatal(err) }
    if term.ID.String()=="" { t.Fatal("expected term id") }
}

func TestTermOverlapRejected(t *testing.T) {
    s := academicTestService(t)
    start := time.Date(2026,9,1,0,0,0,0,time.UTC)
    end := time.Date(2027,7,31,0,0,0,0,time.UTC)
    session, err := s.CreateSession(models.AcademicSession{Name:"2027/2028",StartDate:start,EndDate:end})
    if err != nil { t.Fatal(err) }
    _, err = s.CreateTerm(models.Term{AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:start,EndDate:time.Date(2026,12,20,0,0,0,0,time.UTC)})
    if err != nil { t.Fatal(err) }
    _, err = s.CreateTerm(models.Term{AcademicSessionID:session.ID,Name:models.TermSecond,StartDate:time.Date(2026,12,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,3,20,0,0,0,0,time.UTC)})
    if err != ErrAcademicOverlap { t.Fatalf("expected overlap error, got %v",err) }
}

func TestOnlyOneActiveSession(t *testing.T) {
    s := academicTestService(t)
    a := time.Date(2026,9,1,0,0,0,0,time.UTC)
    b := time.Date(2027,7,31,0,0,0,0,time.UTC)
    one, err := s.CreateSession(models.AcademicSession{Name:"A",StartDate:a,EndDate:b,Status:models.AcademicStatusActive})
    if err != nil { t.Fatal(err) }
    _, err = s.CreateSession(models.AcademicSession{Name:"B",StartDate:a,EndDate:b,Status:models.AcademicStatusActive})
    if err != nil { t.Fatal(err) }
    old, err := s.GetSession(one.ID)
    if err != nil { t.Fatal(err) }
    if old.Status == models.AcademicStatusActive { t.Fatal("expected previous active session to be closed") }
}
