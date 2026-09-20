package models

import ("time"; "github.com/google/uuid"; "gorm.io/gorm")
const(AttendancePresent="present";AttendanceAbsent="absent";AttendanceLate="late";AttendanceExcused="excused")
type AttendanceRecord struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_attendance_enrollment_date,priority:1" json:"school_id"`
 School School `gorm:"foreignKey:SchoolID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
 EnrollmentID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_attendance_enrollment_date,priority:2" json:"enrollment_id"`
 TermID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:uq_school_attendance_enrollment_date,priority:3" json:"term_id"`
 Date time.Time `gorm:"type:date;not null;uniqueIndex:uq_school_attendance_enrollment_date,priority:4" json:"date"`
 Status string `gorm:"size:20;not null;index" json:"status"`
 Note string `gorm:"size:500" json:"note"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
 Enrollment StudentEnrollment `gorm:"foreignKey:EnrollmentID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"enrollment,omitempty"`
 Term Term `gorm:"foreignKey:TermID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"term,omitempty"`
}
func(a *AttendanceRecord)BeforeCreate(tx *gorm.DB)error{a.ID=uuid.New();if a.Status==""{a.Status=AttendancePresent};return nil}