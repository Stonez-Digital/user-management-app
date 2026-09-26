package models

import (
 "time"
 "github.com/google/uuid"
)

const (
 ContentDraft="draft"
 ContentPublished="published"
 ContentArchived="archived"
)

type BlogPost struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index" json:"school_id"`
 AuthorID uuid.UUID `gorm:"type:uuid;not null" json:"author_id"`
 CategoryID *uuid.UUID `gorm:"type:uuid;index" json:"category_id,omitempty"`
 Title string `gorm:"size:240;not null" json:"title"`
 Slug string `gorm:"size:240;not null" json:"slug"`
 Excerpt string `gorm:"size:500" json:"excerpt"`
 Content string `gorm:"type:text;not null" json:"content"`
 FeaturedImageURL string `gorm:"size:1200" json:"featured_image_url,omitempty"`
 Status string `gorm:"size:20;not null;index" json:"status"`
 Featured bool `gorm:"not null;default:false;index" json:"featured"`
 PublishedAt *time.Time `json:"published_at,omitempty"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}

type BlogCategory struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index" json:"school_id"`
 Name string `gorm:"size:120;not null" json:"name"`
 Slug string `gorm:"size:140;not null" json:"slug"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}

type GalleryAlbum struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index" json:"school_id"`
 Title string `gorm:"size:180;not null" json:"title"`
 Slug string `gorm:"size:200;not null" json:"slug"`
 Description string `gorm:"size:1000" json:"description"`
 CoverImageURL string `gorm:"size:1200" json:"cover_image_url,omitempty"`
 Status string `gorm:"size:20;not null;index" json:"status"`
 Featured bool `gorm:"not null;default:false;index" json:"featured"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}

type GalleryImage struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index" json:"school_id"`
 AlbumID uuid.UUID `gorm:"type:uuid;not null;index" json:"album_id"`
 UploadedBy uuid.UUID `gorm:"type:uuid;not null" json:"uploaded_by"`
 ImageURL string `gorm:"size:1200;not null" json:"image_url"`
 Caption string `gorm:"size:300" json:"caption"`
 AltText string `gorm:"size:300" json:"alt_text"`
 DisplayOrder int `gorm:"not null;default:0;index" json:"display_order"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}


type SchoolAdvertisement struct {
 ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
 SchoolID uuid.UUID `gorm:"type:uuid;not null;index" json:"school_id"`
 Title string `gorm:"size:240;not null" json:"title"`
 Description string `gorm:"size:1000" json:"description"`
 ImageURL string `gorm:"size:1200;not null" json:"image_url"`
 ButtonText string `gorm:"size:80" json:"button_text,omitempty"`
 TargetURL string `gorm:"size:1200" json:"target_url,omitempty"`
 Status string `gorm:"size:20;not null;index" json:"status"`
 Featured bool `gorm:"not null;default:true;index" json:"featured"`
 StartsAt *time.Time `json:"starts_at,omitempty"`
 EndsAt *time.Time `json:"ends_at,omitempty"`
 CreatedBy uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
}
