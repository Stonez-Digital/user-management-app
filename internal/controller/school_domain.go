package controller

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"gorm.io/gorm"
)

var reservedSchoolSlugs = map[string]bool{
	"app": true, "api": true, "www": true, "mail": true, "smtp": true,
	"ftp": true, "admin": true, "autodiscover": true,
}

// uniqueSchoolSlug reserves a DNS-safe, name-first hostname. A school code is
// appended only when the school name is already taken or is a reserved host.
func uniqueSchoolSlug(tx *gorm.DB, name, code string, excludeID uuid.UUID) (string, error) {
	base := models.SchoolSlug(name, "")
	if len(base) > 63 {
		base = strings.Trim(base[:63], "-")
	}
	if base == "" {
		base = "school"
	}
	exists := func(candidate string) (bool, error) {
		q := tx.Model(&models.School{}).Where("lower(slug) = ?", strings.ToLower(candidate))
		if excludeID != uuid.Nil {
			q = q.Where("id <> ?", excludeID)
		}
		var count int64
		if err := q.Count(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	}
	taken, err := exists(base)
	if err != nil {
		return "", err
	}
	if !taken && !reservedSchoolSlugs[base] {
		return base, nil
	}

	suffix := models.SchoolSlug(code, "")
	if suffix == "school" {
		return "", fmt.Errorf("school name requires a unique subdomain suffix")
	}
	if len(suffix) > 24 {
		suffix = suffix[:24]
	}
	maxBase := 63 - len(suffix) - 1
	if maxBase < 1 {
		return "", fmt.Errorf("school code cannot form a valid subdomain")
	}
	if len(base) > maxBase {
		base = strings.Trim(base[:maxBase], "-")
	}
	candidate := strings.Trim(base, "-") + "-" + suffix
	taken, err = exists(candidate)
	if err != nil {
		return "", err
	}
	if taken || reservedSchoolSlugs[candidate] {
		return "", gorm.ErrDuplicatedKey
	}
	return candidate, nil
}

func validateRequestedSchoolSlug(slug string) error {
	if len(slug) < 3 || len(slug) > 63 {
		return fmt.Errorf("subdomain must be between 3 and 63 characters")
	}
	if slug[0] == '-' || slug[len(slug)-1] == '-' {
		return fmt.Errorf("subdomain must start and end with a letter or number")
	}
	for _, ch := range slug {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-') {
			return fmt.Errorf("subdomain may contain only lowercase letters, numbers, and hyphens")
		}
	}
	if reservedSchoolSlugs[slug] {
		return fmt.Errorf("that subdomain is reserved")
	}
	return nil
}

func requestedSchoolSlugAvailable(tx *gorm.DB, slug string) (bool, error) {
	var count int64
	if err := tx.Model(&models.School{}).Where("lower(slug) = ?", strings.ToLower(slug)).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}
