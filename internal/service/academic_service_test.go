package service

import (
    "testing"
    "github.com/google/uuid"
    "time"

    "github.com/onoja217/users-management-app/internal/models"
    "github.com/onoja217/users-management-app/internal/repository"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func academicTestService(t *testing.T) *AcademicService {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file:academic_test_"+uuid.New().String()+"?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.AcademicSession{}, &models.Term{}); err != nil { t.Fatal(err) }
    return NewAcademicService(repository.NewAcademicSessionRepository(db), repository.NewTermRepository(db), db)
}

func TestCreateSessionAndTerm(t *testing.T) {
    _ = uuid.Nil
    s := academicTestService(t)
    start := time.Date(2026,9,1,0,0,0,0,time.UTC)
    end := time.Date(2027,7,31,0,0,0,0,time.UTC)
    session, err := s.CreateSession(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.AcademicSession{SchoolID:uuid.MustParse("00000000-0000-0000-0000-000000000001"),Name:"2026/2027",StartDate:start,EndDate:end})
    if err != nil { t.Fatal(err) }
    if session.Status != models.AcademicStatusPlanned { t.Fatalf("expected planned, got %s",session.Status) }

    term, err := s.CreateTerm(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.Term{AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:start,EndDate:time.Date(2026,12,20,0,0,0,0,time.UTC)})
    if err != nil { t.Fatal(err) }
    if term.ID.String()=="" { t.Fatal("expected term id") }
}

func TestTermOverlapRejected(t *testing.T) {
    s := academicTestService(t)
    start := time.Date(2026,9,1,0,0,0,0,time.UTC)
    end := time.Date(2027,7,31,0,0,0,0,time.UTC)
    session, err := s.CreateSession(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.AcademicSession{Name:"2027/2028",StartDate:start,EndDate:end})
    if err != nil { t.Fatal(err) }
    _, err = s.CreateTerm(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.Term{AcademicSessionID:session.ID,Name:models.TermFirst,StartDate:start,EndDate:time.Date(2026,12,20,0,0,0,0,time.UTC)})
    if err != nil { t.Fatal(err) }
    _, err = s.CreateTerm(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.Term{AcademicSessionID:session.ID,Name:models.TermSecond,StartDate:time.Date(2026,12,1,0,0,0,0,time.UTC),EndDate:time.Date(2027,3,20,0,0,0,0,time.UTC)})
    if err != ErrAcademicOverlap { t.Fatalf("expected overlap error, got %v",err) }
}

func TestOnlyOneActiveSession(t *testing.T) {
    s := academicTestService(t)
    a := time.Date(2026,9,1,0,0,0,0,time.UTC)
    b := time.Date(2027,7,31,0,0,0,0,time.UTC)
    one, err := s.CreateSession(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.AcademicSession{SchoolID:uuid.MustParse("00000000-0000-0000-0000-000000000001"),Name:"A",StartDate:a,EndDate:b,Status:models.AcademicStatusActive})
    if err != nil { t.Fatal(err) }
    _, err = s.CreateSession(uuid.MustParse("00000000-0000-0000-0000-000000000001"),models.AcademicSession{SchoolID:uuid.MustParse("00000000-0000-0000-0000-000000000001"),Name:"B",StartDate:a,EndDate:b,Status:models.AcademicStatusActive})
    if err != nil { t.Fatal(err) }
    old, err := s.GetSession(uuid.MustParse("00000000-0000-0000-0000-000000000001"),one.ID)
    if err != nil { t.Fatal(err) }
    if old.Status == models.AcademicStatusActive { t.Fatal("expected previous active session to be closed") }
}


func TestAcademicSessionsAreIsolatedBySchool(t *testing.T) {
    s := academicTestService(t)
    firstSchool := uuid.MustParse("00000000-0000-0000-0000-000000000001")
    secondSchool := uuid.MustParse("00000000-0000-0000-0000-000000000002")
    start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
    end := time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC)

    first, err := s.CreateSession(firstSchool, models.AcademicSession{Name: "2026/2027", StartDate: start, EndDate: end})
    if err != nil { t.Fatal(err) }
    second, err := s.CreateSession(secondSchool, models.AcademicSession{Name: "2026/2027", StartDate: start, EndDate: end})
    if err != nil { t.Fatal(err) }

    firstSessions, err := s.GetSessions(firstSchool)
    if err != nil { t.Fatal(err) }
    secondSessions, err := s.GetSessions(secondSchool)
    if err != nil { t.Fatal(err) }

    if len(firstSessions) != 1 || firstSessions[0].ID != first.ID { t.Fatalf("unexpected first-school sessions: %+v", firstSessions) }
    if len(secondSessions) != 1 || secondSessions[0].ID != second.ID { t.Fatalf("unexpected second-school sessions: %+v", secondSessions) }

    if _, err := s.GetSession(firstSchool, second.ID); err != ErrAcademicSessionNotFound {
        t.Fatalf("expected cross-school session lookup to be hidden, got %v", err)
    }
}


func financeTestService(t *testing.T) *FinanceService {
    t.Helper()
    db, err := gorm.Open(sqlite.Open("file:finance_test_"+uuid.New().String()+"?mode=memory&cache=shared"), &gorm.Config{})
    if err != nil { t.Fatal(err) }
    if err := db.AutoMigrate(&models.School{}, &models.AcademicSession{}, &models.Term{}, &models.FeeItem{}); err != nil { t.Fatal(err) }
    return NewFinanceService(repository.NewFeeItemRepository(db), nil, nil, db)
}

func financeSchoolAndTerm(t *testing.T, db *gorm.DB, schoolID uuid.UUID, name string) models.Term {
    t.Helper()
    if err := db.Create(&models.School{ID: schoolID, Name: name, Code: name}).Error; err != nil { t.Fatal(err) }
    session := models.AcademicSession{SchoolID: schoolID, Name: "2026/2027-" + name, StartDate: time.Date(2026,9,1,0,0,0,0,time.UTC), EndDate: time.Date(2027,7,31,0,0,0,0,time.UTC)}
    if err := db.Create(&session).Error; err != nil { t.Fatal(err) }
    term := models.Term{SchoolID: schoolID, AcademicSessionID: session.ID, Name: models.TermFirst, StartDate: session.StartDate, EndDate: time.Date(2026,12,20,0,0,0,0,time.UTC)}
    if err := db.Create(&term).Error; err != nil { t.Fatal(err) }
    return term
}

func TestFinanceFeeItemsAreIsolatedBySchool(t *testing.T) {
    svc := financeTestService(t)
    db := svc.DB()
    schoolA, schoolB := uuid.New(), uuid.New()
    termA := financeSchoolAndTerm(t, db, schoolA, "School A")
    termB := financeSchoolAndTerm(t, db, schoolB, "School B")

    feeA, err := svc.CreateFee(schoolA, models.FeeItem{TermID: termA.ID, Name: "Tuition", Amount: 50000, Active: true})
    if err != nil { t.Fatal(err) }
    feeB, err := svc.CreateFee(schoolB, models.FeeItem{TermID: termB.ID, Name: "Tuition", Amount: 60000, Active: true})
    if err != nil { t.Fatal(err) }

    feesA, err := svc.ListFees(schoolA, nil)
    if err != nil { t.Fatal(err) }
    if len(feesA) != 1 || feesA[0].ID != feeA.ID || feesA[0].Amount != 50000 { t.Fatalf("unexpected School A fees: %+v", feesA) }
    feesB, err := svc.ListFees(schoolB, nil)
    if err != nil { t.Fatal(err) }
    if len(feesB) != 1 || feesB[0].ID != feeB.ID || feesB[0].Amount != 60000 { t.Fatalf("unexpected School B fees: %+v", feesB) }

    if _, err := svc.GetFee(schoolA, feeB.ID); err != ErrFeeNotFound { t.Fatalf("expected cross-school fee lookup to be hidden, got %v", err) }
}

func TestFinanceFeeDuplicateCheckIsSchoolScoped(t *testing.T) {
    svc := financeTestService(t)
    db := svc.DB()
    schoolA, schoolB := uuid.New(), uuid.New()
    termA := financeSchoolAndTerm(t, db, schoolA, "School C")
    termB := financeSchoolAndTerm(t, db, schoolB, "School D")

    if _, err := svc.CreateFee(schoolA, models.FeeItem{TermID: termA.ID, Name: "Uniform", Amount: 10000}); err != nil { t.Fatal(err) }
    if _, err := svc.CreateFee(schoolB, models.FeeItem{TermID: termB.ID, Name: "Uniform", Amount: 12000}); err != nil { t.Fatalf("same fee name should be allowed in another school: %v", err) }
    if _, err := svc.CreateFee(schoolA, models.FeeItem{TermID: termA.ID, Name: " uniform ", Amount: 11000}); err != ErrFeeDuplicate { t.Fatalf("expected same-school duplicate rejection, got %v", err) }
}

func TestFinanceFeeMutationCannotCrossSchool(t *testing.T) {
    svc := financeTestService(t)
    db := svc.DB()
    schoolA, schoolB := uuid.New(), uuid.New()
    termA := financeSchoolAndTerm(t, db, schoolA, "School E")
    fee, err := svc.CreateFee(schoolA, models.FeeItem{TermID: termA.ID, Name: "Exam", Amount: 5000})
    if err != nil { t.Fatal(err) }

    cross := fee
    cross.Name, cross.Amount = "Changed", 9000
    if err := svc.UpdateFee(schoolB, cross); err != ErrFeeNotFound { t.Fatalf("expected cross-school update to be blocked, got %v", err) }
    if err := svc.DeleteFee(schoolB, fee.ID); err != ErrFeeNotFound { t.Fatalf("expected cross-school delete to be blocked, got %v", err) }
    if _, err := svc.GetFee(schoolA, fee.ID); err != nil { t.Fatal(err) }
}
