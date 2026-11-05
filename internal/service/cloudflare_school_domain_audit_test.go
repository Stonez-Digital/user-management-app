package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListAttachedDomainsReadsEveryPageWithoutMutation(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet {
			t.Fatalf("audit must be read-only, got %s", r.Method)
		}
		if r.URL.Path != "/accounts/account-123/workers/domains" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("expected bearer token")
		}
		if r.URL.Query().Get("service") != "school-worker" || r.URL.Query().Get("per_page") != "100" {
			t.Fatalf("missing worker filter or page size: %s", r.URL.RawQuery)
		}
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		if page == "1" {
			_, _ = fmt.Fprint(w, `{"success":true,"errors":[],"result":[{"hostname":"one.example.test","service":"school-worker","enabled":true}],"result_info":{"total_pages":2}}`)
			return
		}
		if page == "2" {
			_, _ = fmt.Fprint(w, `{"success":true,"errors":[],"result":[{"hostname":"two.example.test","service":"school-worker","enabled":false}],"result_info":{"total_pages":2}}`)
			return
		}
		t.Fatalf("unexpected page %q", page)
	}))
	defer server.Close()

	provisioner := &CloudflareSchoolDomainProvisioner{
		token: "test-token", accountID: "account-123", workerName: "school-worker",
		baseDomain: "example.test", apiBaseURL: server.URL, client: server.Client(),
	}
	domains, err := provisioner.ListAttachedDomains(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(domains) != 2 {
		t.Fatalf("expected two read-only pages, requests=%d domains=%d", requests, len(domains))
	}
	if domains[0].Hostname != "one.example.test" || !domains[0].Enabled || domains[1].Hostname != "two.example.test" || domains[1].Enabled {
		t.Fatalf("unexpected domain results: %+v", domains)
	}
}

func TestListAttachedDomainsFailsClosedOnCloudflareError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"success":false,"errors":[{"code":10000,"message":"permission denied"}],"result":[]}`)
	}))
	defer server.Close()
	provisioner := &CloudflareSchoolDomainProvisioner{
		token: "test-token", accountID: "account-123", workerName: "school-worker",
		apiBaseURL: server.URL, client: server.Client(),
	}
	if _, err := provisioner.ListAttachedDomains(context.Background()); err == nil {
		t.Fatal("expected permission error to stop the audit")
	}
}
