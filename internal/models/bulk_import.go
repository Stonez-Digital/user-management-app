package models

import ("time"; "github.com/google/uuid"; "gorm.io/gorm")

const (
 BulkImportPending="pending"
 BulkImportValidating="validating"
 BulkImportQueued="queued"
 BulkImportRunning="running"
 BulkImportCompleted="completed"
 BulkImportCompletedWithErrors="completed_with_errors"
 BulkImportFailed="failed"
)

type BulkImportJob struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index" json:"school_id"`
 InitiatedBy uuid.UUID `gorm:"type:uuid;not null;index" json:"initiated_by"`
 Kind string `gorm:"size:20;not null;index" json:"kind"`
 FileName string `gorm:"size:255;not null" json:"file_name"`
 Status string `gorm:"size:40;not null;index" json:"status"`
 Total int `gorm:"not null;default:0" json:"total"`
 Valid int `gorm:"not null;default:0" json:"valid"`
 Created int `gorm:"not null;default:0" json:"created"`
 Updated int `gorm:"not null;default:0" json:"updated"`
 Skipped int `gorm:"not null;default:0" json:"skipped"`
 Failed int `gorm:"not null;default:0" json:"failed"`
 RowsJSON string `gorm:"type:text" json:"-"`
 ErrorsJSON string `gorm:"type:text" json:"-"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
func (j *BulkImportJob) BeforeCreate(tx *gorm.DB) error { j.ID=uuid.New(); return nil }
