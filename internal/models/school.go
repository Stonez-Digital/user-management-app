package models

import (
    "time"
    "strings"
    "unicode"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

const (
    SchoolStatusPending = "pending"
    SchoolStatusActive = "active"
    SchoolStatusSuspended = "suspended"
)

type School struct {
    ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    Name string `gorm:"size:160;not null" json:"name"`
    Code string `gorm:"size:50;not null;uniqueIndex" json:"code"`
    // Slug uniqueness is owned by the explicit database migration so production
    // PostgreSQL uses a case-insensitive partial unique index.
    Slug string `gorm:"size:180" json:"slug"`
    LogoURL string `gorm:"size:1000" json:"logo_url,omitempty"`
    Status string `gorm:"size:20;not null;default:active;index" json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Users []User `gorm:"foreignKey:SchoolID" json:"-"`
}

func SchoolSlug(name, code string) string {
    var b strings.Builder
    lastDash := false
    for _, r := range strings.ToLower(strings.TrimSpace(name)) {
        if unicode.IsLetter(r) || unicode.IsDigit(r) { b.WriteRune(r); lastDash = false; continue }
        if !lastDash { b.WriteByte('-'); lastDash = true }
    }
    slug := strings.Trim(b.String(), "-")
    if slug == "" { slug = "school" }
    if code = strings.Trim(strings.ToLower(code), "- "); code != "" { slug += "-" + code }
    return slug
}

func (s *School) BeforeCreate(tx *gorm.DB) error {
    if s.ID == uuid.Nil { s.ID = uuid.New() }
    return nil
}
