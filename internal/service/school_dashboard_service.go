package service

import (
    "time"

    "github.com/google/uuid"
    "github.com/onoja217/users-management-app/internal/models"
    "gorm.io/gorm"
)

type SchoolDashboardService struct{ db *gorm.DB }

func NewSchoolDashboardService(db *gorm.DB) *SchoolDashboardService {
    return &SchoolDashboardService{db: db}
}

type SchoolDashboard struct {
    School          SchoolDashboardIdentity  `json:"school"`
    AcademicContext SchoolDashboardAcademic  `json:"academic_context"`
    Overview        SchoolDashboardOverview  `json:"overview"`
    Attendance      SchoolDashboardAttendance `json:"attendance"`
    Finance         SchoolDashboardFinance   `json:"finance"`
    Alerts          []SchoolDashboardAlert   `json:"alerts"`
    RecentActivity  []SchoolDashboardActivity `json:"recent_activity"`
    Health          SchoolDashboardHealth    `json:"health"`
}

type SchoolDashboardIdentity struct {
    ID           uuid.UUID `json:"id"`
    Name         string    `json:"name"`
    LogoURL      string    `json:"logo_url,omitempty"`
    Address      string    `json:"address,omitempty"`
    ContactEmail string    `json:"contact_email,omitempty"`
    ContactPhone string    `json:"contact_phone,omitempty"`
    Administrator SchoolDashboardPerson `json:"administrator"`
}

type SchoolDashboardPerson struct {
    ID    uuid.UUID `json:"id"`
    Name  string    `json:"name"`
    Email string    `json:"email"`
}

type SchoolDashboardAcademic struct {
    SessionID   uuid.UUID `json:"session_id,omitempty"`
    SessionName string    `json:"session_name,omitempty"`
    TermID      uuid.UUID `json:"term_id,omitempty"`
    TermName    string    `json:"term_name,omitempty"`
}

type SchoolDashboardOverview struct {
    Students          int64 `json:"students"`
    ActiveEnrollments int64 `json:"active_enrollments"`
    Teachers          int64 `json:"teachers"`
    Parents           int64 `json:"parents"`
    Classes           int64 `json:"classes"`
    Sections          int64 `json:"sections"`
    Subjects          int64 `json:"subjects"`
}

type SchoolDashboardAttendance struct {
    ExpectedToday int64   `json:"expected_today"`
    Present       int64   `json:"present"`
    Absent        int64   `json:"absent"`
    Late          int64   `json:"late"`
    Excused       int64   `json:"excused"`
    Percentage    float64 `json:"percentage"`
    Recorded      bool    `json:"recorded"`
}

type SchoolDashboardFinance struct {
    TotalInvoiced       float64                    `json:"total_invoiced"`
    AmountPaid          float64                    `json:"amount_paid"`
    OutstandingBalance  float64                    `json:"outstanding_balance"`
    OutstandingInvoices int64                      `json:"outstanding_invoices"`
    RecentInvoices      []SchoolDashboardInvoice   `json:"recent_invoices"`
    RecentPayments      []SchoolDashboardPayment   `json:"recent_payments"`
}

type SchoolDashboardInvoice struct {
    ID            uuid.UUID `json:"id"`
    InvoiceNumber string    `json:"invoice_number"`
    TotalAmount   float64   `json:"total_amount"`
    PaidAmount    float64   `json:"paid_amount"`
    Balance       float64   `json:"balance"`
    Status        string    `json:"status"`
    CreatedAt     time.Time `json:"created_at"`
}

type SchoolDashboardPayment struct {
    ID            uuid.UUID `json:"id"`
    InvoiceID     uuid.UUID `json:"invoice_id"`
    InvoiceNumber string    `json:"invoice_number"`
    Amount        float64   `json:"amount"`
    Provider      string    `json:"provider"`
    Reference     string    `json:"reference"`
    Status        string    `json:"status"`
    PaidAt        *time.Time `json:"paid_at,omitempty"`
    CreatedAt     time.Time `json:"created_at"`
}

type SchoolDashboardAlert struct {
    Key      string `json:"key"`
    Severity string `json:"severity"`
    Count    int64  `json:"count"`
    Message  string `json:"message"`
    Action   string `json:"action"`
}

type SchoolDashboardActivity struct {
    ID         uuid.UUID  `json:"id"`
    Action     string     `json:"action"`
    Resource   string     `json:"resource"`
    ResourceID *uuid.UUID `json:"resource_id,omitempty"`
    ActorID    *uuid.UUID `json:"actor_id,omitempty"`
    ActorName  string     `json:"actor_name,omitempty"`
    CreatedAt  time.Time  `json:"created_at"`
}

type SchoolDashboardHealth struct {
    AcademicSetup      string `json:"academic_setup"`
    Enrollment         string `json:"enrollment"`
    TeacherAllocation  string `json:"teacher_allocation"`
    AttendanceActivity string `json:"attendance_activity"`
    AssessmentActivity string `json:"assessment_activity"`
    FinanceActivity    string `json:"finance_activity"`
}

func (s *SchoolDashboardService) Get(schoolID, adminID, sessionID, termID uuid.UUID) (SchoolDashboard, error) {
    var school models.School
    if err := s.db.Where("id = ?", schoolID).First(&school).Error; err != nil { return SchoolDashboard{}, err }

    var admin models.User
    if err := s.db.Where("id = ? AND school_id = ? AND role = ?", adminID, schoolID, "school_admin").First(&admin).Error; err != nil { return SchoolDashboard{}, err }

    var session models.AcademicSession
    sessionFound := false
    if sessionID != uuid.Nil {
        sessionFound = s.db.Where("id = ? AND school_id = ?", sessionID, schoolID).First(&session).Error == nil
    }
    if !sessionFound {
        sessionFound = s.db.Where("school_id = ? AND status = ?", schoolID, models.AcademicStatusActive).Order("start_date DESC").First(&session).Error == nil
    }

    var term models.Term
    termFound := false
    if sessionFound && termID != uuid.Nil {
        termFound = s.db.Where("id = ? AND school_id = ? AND academic_session_id = ?", termID, schoolID, session.ID).First(&term).Error == nil
    }
    if sessionFound && !termFound {
        termFound = s.db.Where("school_id = ? AND academic_session_id = ? AND status = ?", schoolID, session.ID, models.AcademicStatusActive).Order("start_date DESC").First(&term).Error == nil
    }

    overview := SchoolDashboardOverview{}
    s.db.Model(&models.Student{}).Where("school_id = ?", schoolID).Count(&overview.Students)
    s.db.Model(&models.User{}).Where("school_id = ? AND role = ? AND active = ?", schoolID, "teacher", true).Count(&overview.Teachers)
    s.db.Model(&models.User{}).Where("school_id = ? AND role = ? AND active = ?", schoolID, "parent", true).Count(&overview.Parents)
    s.db.Model(&models.SchoolClass{}).Where("school_id = ?", schoolID).Count(&overview.Classes)
    s.db.Model(&models.Section{}).Where("school_id = ?", schoolID).Count(&overview.Sections)
    s.db.Model(&models.Subject{}).Where("school_id = ? AND active = ?", schoolID, true).Count(&overview.Subjects)
    if sessionFound {
        s.db.Model(&models.StudentEnrollment{}).Where("school_id = ? AND academic_session_id = ? AND status = ?", schoolID, session.ID, models.EnrollmentStatusActive).Count(&overview.ActiveEnrollments)
    }

    attendance := SchoolDashboardAttendance{}
    if sessionFound {
        s.db.Model(&models.StudentEnrollment{}).Where("school_id = ? AND academic_session_id = ? AND status = ?", schoolID, session.ID, models.EnrollmentStatusActive).Count(&attendance.ExpectedToday)
    }
    if termFound {
        today := time.Now().UTC().Truncate(24 * time.Hour)
        var records []models.AttendanceRecord
        s.db.Where("school_id = ? AND term_id = ? AND date = ?", schoolID, term.ID, today).Find(&records)
        attendance.Recorded = len(records) > 0
        for _, r := range records {
            switch r.Status {
            case models.AttendancePresent:
                attendance.Present++
            case models.AttendanceAbsent:
                attendance.Absent++
            case models.AttendanceLate:
                attendance.Late++
            case models.AttendanceExcused:
                attendance.Excused++
            }
        }
        if attendance.ExpectedToday > 0 {
            attendance.Percentage = float64(attendance.Present+attendance.Late) / float64(attendance.ExpectedToday) * 100
        }
    }

    finance := SchoolDashboardFinance{RecentInvoices: []SchoolDashboardInvoice{}, RecentPayments: []SchoolDashboardPayment{}}
    if termFound {
        var invs []models.Invoice
        if err := s.db.Where("school_id = ? AND term_id = ?", schoolID, term.ID).Order("created_at DESC").Find(&invs).Error; err != nil { return SchoolDashboard{}, err }
        for _, inv := range invs {
            finance.TotalInvoiced += inv.TotalAmount
            finance.AmountPaid += inv.PaidAmount
            finance.OutstandingBalance += inv.Balance
            if inv.Balance > 0 && inv.Status != models.InvoiceStatusCancelled { finance.OutstandingInvoices++ }
            if len(finance.RecentInvoices) < 5 {
                finance.RecentInvoices = append(finance.RecentInvoices, SchoolDashboardInvoice{inv.ID, inv.InvoiceNumber, inv.TotalAmount, inv.PaidAmount, inv.Balance, inv.Status, inv.CreatedAt})
            }
        }
        var pays []models.Payment
        if err := s.db.Where("payments.school_id = ?", schoolID).Joins("JOIN invoices ON invoices.id = payments.invoice_id AND invoices.school_id = ? AND invoices.term_id = ?", schoolID, term.ID).Order("payments.created_at DESC").Limit(5).Find(&pays).Error; err != nil { return SchoolDashboard{}, err }
        for _, p := range pays {
            var inv models.Invoice
            _ = s.db.Select("invoice_number").Where("id = ? AND school_id = ?", p.InvoiceID, schoolID).First(&inv).Error
            finance.RecentPayments = append(finance.RecentPayments, SchoolDashboardPayment{p.ID, p.InvoiceID, inv.InvoiceNumber, p.Amount, p.Provider, p.Reference, p.Status, p.PaidAt, p.CreatedAt})
        }
    }

    alerts := make([]SchoolDashboardAlert, 0)
    if !sessionFound {
        alerts = append(alerts, SchoolDashboardAlert{"active_session_missing","critical",1,"No active academic session is configured.","/dashboard/academic"})
    } else if !termFound {
        alerts = append(alerts, SchoolDashboardAlert{"active_term_missing","critical",1,"No active term is configured for the selected session.","/dashboard/academic"})
    }

    var count int64
    if sessionFound {
        s.db.Model(&models.Student{}).Where("students.school_id = ? AND NOT EXISTS (SELECT 1 FROM student_enrollments WHERE student_enrollments.school_id = students.school_id AND student_enrollments.student_id = students.id AND student_enrollments.academic_session_id = ? AND student_enrollments.status = ?)", schoolID, session.ID, models.EnrollmentStatusActive).Count(&count)
        if count > 0 { alerts = append(alerts, SchoolDashboardAlert{"students_without_enrollment","warning",count,"students have no active enrollment in the selected session.","/dashboard/enrollments"}) }
    }
    s.db.Model(&models.Student{}).Where("school_id = ? AND (admission_number = '' OR user_id IS NULL)", schoolID).Count(&count)
    if count > 0 { alerts = append(alerts, SchoolDashboardAlert{"student_profile_gaps","warning",count,"students are missing required profile information.","/dashboard/students"}) }

    s.db.Model(&models.User{}).Where("school_id = ? AND role = ? AND active = ? AND (name = '' OR email = '')", schoolID, "teacher", true).Count(&count)
    if count > 0 { alerts = append(alerts, SchoolDashboardAlert{"teacher_profile_gaps","warning",count,"teachers are missing required profile information.","/dashboard/onboarding"}) }

    s.db.Model(&models.User{}).Where("school_id = ? AND role = ? AND active = ? AND (name = '' OR email = '')", schoolID, "parent", true).Count(&count)
    if count > 0 { alerts = append(alerts, SchoolDashboardAlert{"parent_profile_gaps","warning",count,"parents are missing required profile information.","/dashboard/onboarding"}) }

    s.db.Model(&models.Student{}).Where("students.school_id = ? AND NOT EXISTS (SELECT 1 FROM guardian_relationships WHERE guardian_relationships.school_id = students.school_id AND guardian_relationships.student_id = students.id AND guardian_relationships.active = ?)", schoolID, true).Count(&count)
    if count > 0 { alerts = append(alerts, SchoolDashboardAlert{"students_without_guardian","info",count,"students have no active guardian link.","/dashboard/onboarding"}) }

    if sessionFound && termFound {
        s.db.Model(&models.User{}).Where("users.school_id = ? AND users.role = ? AND users.active = ? AND NOT EXISTS (SELECT 1 FROM teacher_assignments WHERE teacher_assignments.school_id = users.school_id AND teacher_assignments.teacher_id = users.id AND teacher_assignments.academic_session_id = ? AND teacher_assignments.term_id = ? AND teacher_assignments.active = ?)", schoolID, "teacher", true, session.ID, term.ID, true).Count(&count)
        if count > 0 { alerts = append(alerts, SchoolDashboardAlert{"teachers_without_assignments","warning",count,"teachers have no assignment in the selected academic context.","/dashboard/teacher-assignments"}) }

        s.db.Model(&models.SchoolClass{}).Where("school_id = ? AND NOT EXISTS (SELECT 1 FROM sections WHERE sections.school_id = school_classes.school_id AND sections.class_id = school_classes.id)", schoolID).Count(&count)
        if count > 0 { alerts = append(alerts, SchoolDashboardAlert{"classes_without_sections","warning",count,"classes have no sections configured.","/dashboard/academic"}) }

        s.db.Model(&models.Section{}).Where("school_id = ? AND NOT EXISTS (SELECT 1 FROM student_enrollments WHERE student_enrollments.school_id = sections.school_id AND student_enrollments.section_id = sections.id AND student_enrollments.academic_session_id = ? AND student_enrollments.status = ?)", schoolID, session.ID, models.EnrollmentStatusActive).Count(&count)
        if count > 0 { alerts = append(alerts, SchoolDashboardAlert{"sections_without_students","info",count,"sections have no active students in the selected session.","/dashboard/enrollments"}) }

        s.db.Model(&models.Subject{}).Where("school_id = ? AND active = ? AND NOT EXISTS (SELECT 1 FROM teacher_assignments WHERE teacher_assignments.school_id = subjects.school_id AND teacher_assignments.subject_id = subjects.id AND teacher_assignments.academic_session_id = ? AND teacher_assignments.term_id = ? AND teacher_assignments.active = ?)", schoolID, true, session.ID, term.ID, true).Count(&count)
        if count > 0 { alerts = append(alerts, SchoolDashboardAlert{"subjects_without_assignments","warning",count,"subjects have no assigned teacher in the selected academic context.","/dashboard/teacher-assignments"}) }

        s.db.Model(&models.Assessment{}).Where("assessments.school_id = ? AND NOT EXISTS (SELECT 1 FROM assessment_results WHERE assessment_results.school_id = assessments.school_id AND assessment_results.assessment_id = assessments.id)", schoolID).Joins("JOIN teacher_assignments ON teacher_assignments.id = assessments.teacher_assignment_id AND teacher_assignments.academic_session_id = ? AND teacher_assignments.term_id = ?", session.ID, term.ID).Count(&count)
        if count > 0 { alerts = append(alerts, SchoolDashboardAlert{"assessments_without_results","info",count,"assessments have no submitted results yet.","/dashboard/results"}) }
    }

    if finance.OutstandingInvoices > 0 {
        alerts = append(alerts, SchoolDashboardAlert{"outstanding_invoices","info",finance.OutstandingInvoices,"invoices have an outstanding balance.","/dashboard/finance"})
    }

    activity := make([]SchoolDashboardActivity, 0, 8)
    var logs []models.AuditLog
    if err := s.db.Where("audit_logs.school_id = ?", schoolID).Order("created_at DESC").Limit(8).Find(&logs).Error; err != nil { return SchoolDashboard{}, err }
    for _, log := range logs {
        var actor models.User
        actorName := ""
        if log.ActorID != nil && s.db.Select("id,name").Where("id = ? AND school_id = ?", *log.ActorID, schoolID).First(&actor).Error == nil {
            actorName = actor.Name
        }
        activity = append(activity, SchoolDashboardActivity{log.ID, log.Action, log.Resource, log.ResourceID, log.ActorID, actorName, log.CreatedAt})
    }

    health := SchoolDashboardHealth{
        AcademicSetup: "Not started",
        Enrollment: "Not started",
        TeacherAllocation: "Not started",
        AttendanceActivity: "Not started",
        AssessmentActivity: "Not started",
        FinanceActivity: "Not started",
    }
    if sessionFound && termFound { health.AcademicSetup = "Complete" } else if sessionFound { health.AcademicSetup = "Needs attention" }
    if overview.Students > 0 && overview.ActiveEnrollments == overview.Students { health.Enrollment = "Complete" } else if overview.Students > 0 { health.Enrollment = "Needs attention" }
    if sessionFound && termFound {
        var assigned int64
        s.db.Model(&models.TeacherAssignment{}).Where("school_id = ? AND academic_session_id = ? AND term_id = ? AND active = ?", schoolID, session.ID, term.ID, true).Count(&assigned)
        if overview.Subjects > 0 && assigned >= overview.Subjects { health.TeacherAllocation = "Complete" } else if assigned > 0 { health.TeacherAllocation = "Needs attention" }
        if attendance.Recorded { health.AttendanceActivity = "Complete" }
        var assessments int64
        s.db.Model(&models.Assessment{}).Joins("JOIN teacher_assignments ON teacher_assignments.id = assessments.teacher_assignment_id").Where("assessments.school_id = ? AND teacher_assignments.academic_session_id = ? AND teacher_assignments.term_id = ?", schoolID, session.ID, term.ID).Count(&assessments)
        if assessments > 0 { health.AssessmentActivity = "Complete" }
        if finance.TotalInvoiced > 0 { health.FinanceActivity = "Complete" }
    }

    return SchoolDashboard{
        School: SchoolDashboardIdentity{school.ID, school.Name, school.LogoURL, school.Address, school.ContactEmail, school.ContactPhone, SchoolDashboardPerson{admin.ID, admin.Name, admin.Email}},
        AcademicContext: SchoolDashboardAcademic{},
        Overview: overview,
        Attendance: attendance,
        Finance: finance,
        Alerts: alerts,
        RecentActivity: activity,
        Health: health,
    }, nil
}
