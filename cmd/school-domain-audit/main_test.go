package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/onoja217/users-management-app/internal/models"
	"github.com/onoja217/users-management-app/internal/service"
)

func TestAuditProposesSafeSlugAndDoesNotClaimUnattachedDomain(t *testing.T) {
	id := uuid.New()
	school := models.School{
		ID: id, Name: "God's Will Secondary and Primary School",
		Code: "GODWILL-001", Slug: "god's will secondary and primary school",
		Status: models.SchoolStatusActive,
	}
	report := buildAuditReport([]models.School{school}, nil, "stonezdigital.com.ng", "stonez-school-management")
	if report.InvalidSlugCount != 1 || report.ActiveSchoolCount != 1 || report.MissingCount != 1 {
		t.Fatalf("unexpected summary: %+v", report)
	}
	entry := report.Schools[0]
	if entry.SlugValid || entry.SuggestedSlug != "god-s-will-secondary-and-primary-school" {
		t.Fatalf("unexpected slug repair proposal: %+v", entry)
	}
	if entry.ExpectedHostname != "god-s-will-secondary-and-primary-school.stonezdigital.com.ng" {
		t.Fatalf("unexpected proposed hostname: %q", entry.ExpectedHostname)
	}
	if entry.WorkerDomainReady || entry.Action != "review_slug_repair_before_domain_attachment" {
		t.Fatalf("audit must not claim a missing attachment is ready: %+v", entry)
	}
}

func TestAuditAddsCodeSuffixWhenSuggestedSlugIsAlreadyReserved(t *testing.T) {
	id := uuid.New()
	base := models.SchoolSlug("God's Will Secondary and Primary School", "")
	existing := models.School{ID: uuid.New(), Name: "Existing school", Slug: base, Status: models.SchoolStatusActive}
	target := models.School{ID: id, Name: "God's Will Secondary and Primary School", Code: "GODWILL-002", Slug: "invalid slug", Status: models.SchoolStatusPending}
	got := suggestedUniqueSlug(target, []models.School{existing, target})
	want := base + "-godwill-002"
	if got != want {
		t.Fatalf("expected collision-safe slug %q, got %q", want, got)
	}
}

func TestAuditOnlyMarksEnabledAttachmentForConfiguredWorkerReady(t *testing.T) {
	school := models.School{ID: uuid.New(), Name: "Bedrock International", Code: "BEDROCK", Slug: "bedrock-international", Status: models.SchoolStatusActive}
	domains := []service.CloudflareWorkerDomain{
		{Hostname: "bedrock-international.stonezdigital.com.ng", Service: "another-worker", Enabled: true},
		{Hostname: "bedrock-international.stonezdigital.com.ng", Service: "stonez-school-management", Enabled: false},
	}
	report := buildAuditReport([]models.School{school}, domains, "stonezdigital.com.ng", "stonez-school-management")
	if report.ReadyCount != 0 || report.MissingCount != 1 || report.Schools[0].WorkerDomainReady {
		t.Fatalf("wrong Worker attachment was treated as ready: %+v", report)
	}

	report = buildAuditReport([]models.School{school}, []service.CloudflareWorkerDomain{
		{Hostname: "BEDROCK-INTERNATIONAL.STONEZDIGITAL.COM.NG.", Service: "stonez-school-management", Enabled: true},
	}, "stonezdigital.com.ng", "stonez-school-management")
	if report.ReadyCount != 1 || report.MissingCount != 0 || !report.Schools[0].WorkerDomainReady {
		t.Fatalf("valid enabled Worker attachment was not recognized: %+v", report)
	}
}
