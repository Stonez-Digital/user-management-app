package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLookupAttachmentReturnsExactWorkerOwnership(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("hostname") != "old.stonezdigital.com.ng" {
			t.Fatalf("unexpected lookup request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success":true,"result":[]interface{}{map[string]interface{}{
			"id":"domain-123","hostname":"old.stonezdigital.com.ng","service":"stonez-school-management","enabled":true,
		}}})
	}))
	defer server.Close()
	p := &CloudflareSchoolDomainProvisioner{token:"test",accountID:"account",workerName:"stonez-school-management",baseDomain:"stonezdigital.com.ng",apiBaseURL:server.URL,client:server.Client()}
	got, err := p.LookupAttachment(context.Background(), "OLD.stonezdigital.com.ng.")
	if err != nil { t.Fatal(err) }
	if got == nil || got.DomainID != "domain-123" || got.Service != p.workerName || !got.Enabled {
		t.Fatalf("unexpected attachment: %#v", got)
	}
}

func TestDetachAttachmentVerifiesOwnershipAndAbsence(t *testing.T) {
	var lookups atomic.Int32
	var deletes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if lookups.Add(1) == 1 {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"success":true,"result":[]interface{}{map[string]interface{}{
					"id":"domain-123","hostname":"old.stonezdigital.com.ng","service":"stonez-school-management","enabled":true,
				}}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success":true,"result":[]interface{}{}})
		case http.MethodDelete:
			if !strings.HasSuffix(r.URL.Path, "/domain-123") { t.Errorf("unexpected delete path: %s", r.URL.Path) }
			deletes.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success":true})
		default:
			t.Errorf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()
	p := &CloudflareSchoolDomainProvisioner{token:"test",accountID:"account",workerName:"stonez-school-management",baseDomain:"stonezdigital.com.ng",apiBaseURL:server.URL,client:server.Client()}
	attachment := CloudflareSchoolDomainAttachment{DomainID:"domain-123",Hostname:"old.stonezdigital.com.ng",Service:p.workerName,Enabled:true}
	if err := p.DetachAttachment(context.Background(), attachment); err != nil { t.Fatal(err) }
	if deletes.Load() != 1 || lookups.Load() != 2 { t.Fatalf("expected ownership lookup, delete, and absence verification; lookups=%d deletes=%d", lookups.Load(), deletes.Load()) }
}

func TestDetachAttachmentRejectsWrongWorkerWithoutDelete(t *testing.T) {
	var deletes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete { deletes.Add(1) }
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success":true,"result":[]interface{}{map[string]interface{}{
			"id":"domain-123","hostname":"old.stonezdigital.com.ng","service":"another-worker","enabled":true,
		}}})
	}))
	defer server.Close()
	p := &CloudflareSchoolDomainProvisioner{token:"test",accountID:"account",workerName:"stonez-school-management",baseDomain:"stonezdigital.com.ng",apiBaseURL:server.URL,client:server.Client()}
	attachment := CloudflareSchoolDomainAttachment{DomainID:"domain-123",Hostname:"old.stonezdigital.com.ng",Service:"stonez-school-management",Enabled:true}
	if err := p.DetachAttachment(context.Background(), attachment); err == nil { t.Fatal("expected ownership mismatch error") }
	if deletes.Load() != 0 { t.Fatal("wrong Worker attachment must never be deleted") }
}
