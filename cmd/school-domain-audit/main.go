package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/onoja217/users-management-app/internal/database"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
)

type schoolAuditEntry struct {
	SchoolID          string `json:"school_id"`
	SchoolName        string `json:"school_name"`
	Status            string `json:"status"`
	CurrentSlug       string `json:"current_slug"`
	SlugValid         bool   `json:"slug_valid"`
	SuggestedSlug     string `json:"suggested_slug,omitempty"`
	ExpectedHostname  string `json:"expected_hostname,omitempty"`
	WorkerDomainReady bool   `json:"worker_domain_ready"`
	Action            string `json:"action"`
}

type auditReport struct {
	ReadOnly          bool               `json:"read_only"`
	GeneratedAt       time.Time          `json:"generated_at"`
	BaseDomain        string             `json:"base_domain"`
	Worker            string             `json:"worker"`
	SchoolCount       int                `json:"school_count"`
	ActiveSchoolCount int                `json:"active_school_count"`
	ReadyCount        int                `json:"ready_count"`
	MissingCount      int                `json:"missing_worker_attachments"`
	InvalidSlugCount  int                `json:"invalid_slug_count"`
	Schools           []schoolAuditEntry `json:"schools"`
}

var reservedSchoolHosts = map[string]bool{
	"app": true, "api": true, "www": true, "mail": true, "smtp": true,
	"ftp": true, "admin": true, "autodiscover": true,
}

func main() {
	if len(os.Args) > 1 {
		log.Fatal("this command is read-only and accepts no mutation flags")
	}
	if os.Getenv("DB_DRIVER") != "postgres" || strings.TrimSpace(os.Getenv("DATABASE_URL")) == "" {
		log.Fatal("set DB_DRIVER=postgres and DATABASE_URL for the intended database; SQLite is not accepted")
	}
	provisioner := service.NewCloudflareSchoolDomainProvisionerFromEnv()
	if provisioner == nil {
		log.Fatal("CLOUDFLARE_API_TOKEN, CLOUDFLARE_ACCOUNT_ID, and CLOUDFLARE_WORKER_NAME are required")
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	var schools []models.School
	if err := db.Select("id", "name", "code", "slug", "status").Order("created_at ASC").Find(&schools).Error; err != nil {
		log.Fatalf("load school inventory: %v", err)
	}
	domains, err := provisioner.ListAttachedDomains(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	report := buildAuditReport(schools, domains, service.SchoolBaseDomainFromEnv(), os.Getenv("CLOUDFLARE_WORKER_NAME"))
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		log.Fatal(err)
	}
}

func buildAuditReport(schools []models.School, domains []service.CloudflareWorkerDomain, baseDomain, worker string) auditReport {
	report := auditReport{
		ReadOnly: true, GeneratedAt: time.Now().UTC(), BaseDomain: baseDomain,
		Worker: worker, SchoolCount: len(schools), Schools: make([]schoolAuditEntry, 0, len(schools)),
	}
	attached := make(map[string]bool, len(domains))
	for _, domain := range domains {
		if domain.Service == worker && domain.Enabled {
			attached[strings.ToLower(strings.TrimSuffix(domain.Hostname, "."))] = true
		}
	}

	for _, school := range schools {
		entry := schoolAuditEntry{
			SchoolID: school.ID.String(), SchoolName: school.Name, Status: school.Status,
			CurrentSlug: school.Slug, SlugValid: validSchoolDNSLabel(school.Slug),
			Action: "no_change_required",
		}
		if !entry.SlugValid {
			report.InvalidSlugCount++
			entry.SuggestedSlug = suggestedUniqueSlug(school, schools)
			entry.Action = "review_slug_repair_before_domain_attachment"
		}
		if school.Status == models.SchoolStatusActive {
			report.ActiveSchoolCount++
			slug := school.Slug
			if !entry.SlugValid {
				slug = entry.SuggestedSlug
			}
			if validSchoolDNSLabel(slug) {
				entry.ExpectedHostname = strings.ToLower(slug) + "." + baseDomain
				entry.WorkerDomainReady = attached[entry.ExpectedHostname]
			}
			if entry.WorkerDomainReady {
				report.ReadyCount++
				if entry.SlugValid {
					entry.Action = "already_attached"
				}
			} else if entry.SlugValid {
				report.MissingCount++
				entry.Action = "attach_after_review"
			} else {
				report.MissingCount++
			}
		}
		report.Schools = append(report.Schools, entry)
	}
	return report
}

func suggestedUniqueSlug(school models.School, schools []models.School) string {
	base := trimDNSLabel(models.SchoolSlug(school.Name, ""))
	candidate := base
	if reservedSchoolHosts[candidate] || slugTaken(candidate, school.ID, schools) {
		suffix := models.SchoolSlug(school.Code, "")
		if suffix == "school" {
			suffix = strings.ReplaceAll(school.ID.String()[:8], "-", "")
		}
		if len(suffix) > 24 {
			suffix = suffix[:24]
		}
		maxBase := 63 - len(suffix) - 1
		if maxBase < 1 {
			return ""
		}
		candidate = strings.Trim(trimDNSLabel(base[:min(len(base), maxBase)]), "-") + "-" + suffix
	}
	if reservedSchoolHosts[candidate] || slugTaken(candidate, school.ID, schools) {
		suffix := strings.ReplaceAll(school.ID.String()[:8], "-", "")
		maxBase := 63 - len(suffix) - 1
		candidate = strings.Trim(trimDNSLabel(base[:min(len(base), maxBase)]), "-") + "-" + suffix
	}
	if !validSchoolDNSLabel(candidate) || reservedSchoolHosts[candidate] || slugTaken(candidate, school.ID, schools) {
		return ""
	}
	return candidate
}

func slugTaken(candidate string, schoolID interface{ String() string }, schools []models.School) bool {
	for _, existing := range schools {
		if existing.ID.String() != schoolID.String() && strings.EqualFold(existing.Slug, candidate) {
			return true
		}
	}
	return false
}

func trimDNSLabel(value string) string {
	value = strings.Trim(value, "-")
	if len(value) > 63 {
		value = strings.Trim(value[:63], "-")
	}
	if value == "" {
		return "school"
	}
	return value
}

func validSchoolDNSLabel(value string) bool {
	if len(value) == 0 || len(value) > 63 || value != strings.ToLower(value) || value[0] == '-' || value[len(value)-1] == '-' || reservedSchoolHosts[value] {
		return false
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}

func min(a, b int) int {
	if a < b { return a }
	return b
}
