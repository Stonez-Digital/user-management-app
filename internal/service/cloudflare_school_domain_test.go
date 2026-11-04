package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCloudflareSchoolDomainProvisionerAttachesExactHostname(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts/account-123/workers/domains" { t.Fatalf("unexpected path %s", r.URL.Path) }
		if r.Header.Get("Authorization") != "Bearer test-token" { t.Fatal("expected bearer token") }
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil { t.Fatal(err) }
			if body["hostname"] != "bedrock-international.stonezdigital.com.ng" { t.Fatalf("unexpected hostname %q", body["hostname"]) }
			if body["service"] != "stonez-school-management" { t.Fatalf("unexpected Worker %q", body["service"]) }
			_, _ = w.Write([]byte("{\"success\":true,\"errors\":[]}"))
			return
		}
		if r.Method != http.MethodGet { t.Fatalf("expected GET verification, got %s", r.Method) }
		if r.URL.Query().Get("hostname") != "bedrock-international.stonezdigital.com.ng" || r.URL.Query().Get("service") != "stonez-school-management" { t.Fatalf("unexpected verification query: %s", r.URL.RawQuery) }
		_, _ = w.Write([]byte("{\"success\":true,\"result\":[{\"hostname\":\"bedrock-international.stonezdigital.com.ng\",\"service\":\"stonez-school-management\",\"enabled\":true}]}"))
	}))
	defer server.Close()

	provisioner := &CloudflareSchoolDomainProvisioner{
		token:"test-token", accountID:"account-123", workerName:"stonez-school-management",
		baseDomain:"stonezdigital.com.ng", apiBaseURL:server.URL, client:server.Client(),
	}
	if err := provisioner.Attach(context.Background(), "bedrock-international.stonezdigital.com.ng"); err != nil {
		t.Fatalf("expected domain attachment to succeed: %v", err)
	}
}

func TestCloudflareSchoolDomainProvisionerRejectsReservedAndForeignHosts(t *testing.T) {
	provisioner := &CloudflareSchoolDomainProvisioner{
		token:"test-token", accountID:"account-123", workerName:"stonez-school-management",
		baseDomain:"stonezdigital.com.ng", apiBaseURL:"https://example.invalid", client:http.DefaultClient,
	}
	for _, host := range []string{"app.stonezdigital.com.ng", "www.stonezdigital.com.ng", "nested.school.stonezdigital.com.ng", "school.example.com"} {
		if err := provisioner.Attach(context.Background(), host); err == nil {
			t.Fatalf("expected %q to be rejected", host)
		}
	}
}


func TestCloudflareSchoolDomainProvisionerRejectsUnverifiedAttachment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut {
			_, _ = w.Write([]byte("{\"success\":true}"))
			return
		}
		_, _ = w.Write([]byte("{\"success\":true,\"result\":[{\"hostname\":\"bedrock-international.stonezdigital.com.ng\",\"service\":\"another-worker\",\"enabled\":true}]}"))
	}))
	defer server.Close()
	provisioner := &CloudflareSchoolDomainProvisioner{
		token:"test-token", accountID:"account-123", workerName:"stonez-school-management",
		baseDomain:"stonezdigital.com.ng", apiBaseURL:server.URL, client:server.Client(),
	}
	if err := provisioner.Attach(context.Background(), "bedrock-international.stonezdigital.com.ng"); err == nil {
		t.Fatal("expected attachment verification to reject a different Worker")
	}
}
