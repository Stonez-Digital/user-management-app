package bulkcontext

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVaultKVKeySourceLoadsOnlyActiveSigningKey(t *testing.T) {
	activeSecret := []byte("active-vault-test-key-0123456789abcdef")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/secret/data/stonez/bulk-import-context" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Vault-Token") != "test-vault-agent-token" {
			http.Error(w, "unauthorized", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"data":{"data":{"active_key_id":"key-current","active_key":"%s"}}}`, base64.StdEncoding.EncodeToString(activeSecret))
	}))
	defer server.Close()

	tokenFile := filepath.Join(t.TempDir(), "vault-token")
	if err := os.WriteFile(tokenFile, []byte("test-vault-agent-token"), 0600); err != nil {
		t.Fatal(err)
	}
	source, err := NewVaultKVKeySource(server.URL, tokenFile, "secret", "stonez/bulk-import-context", server.Client(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	active, err := source.ActiveSigningKey(t.Context())
	if err != nil {
		t.Fatalf("load active Vault key: %v", err)
	}
	if active.ID != "key-current" || string(active.Secret) != string(activeSecret) {
		t.Fatalf("unexpected active key: ID=%q", active.ID)
	}
	active.Secret[0] = 0
	again, err := source.ActiveSigningKey(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if string(again.Secret) != string(activeSecret) {
		t.Fatal("caller mutation changed cached Vault key material")
	}
}

func TestNewVaultKVKeySourceRejectsUnsafeConfiguration(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("token"), 0600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, address, mount, path string
		cacheTTL time.Duration
	}{
		{name: "plain HTTP rejected", address: "http://vault.example", mount: "secret", path: "app/key", cacheTTL: time.Minute},
		{name: "Vault URL path rejected", address: "https://vault.example/prefix", mount: "secret", path: "app/key", cacheTTL: time.Minute},
		{name: "path traversal rejected", address: "https://vault.example", mount: "secret", path: "../key", cacheTTL: time.Minute},
		{name: "zero cache TTL rejected", address: "https://vault.example", mount: "secret", path: "app/key", cacheTTL: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewVaultKVKeySource(tt.address, tokenFile, tt.mount, tt.path, nil, tt.cacheTTL); err == nil {
				t.Fatal("unsafe Vault configuration was accepted")
			}
		})
	}
}

func TestVaultKVKeySourceRejectsMalformedKeyMaterial(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"data":{"active_key_id":"bad","active_key":"not-base64"}}}`))
	}))
	defer server.Close()
	tokenFile := filepath.Join(t.TempDir(), "vault-token")
	if err := os.WriteFile(tokenFile, []byte("test-token"), 0600); err != nil {
		t.Fatal(err)
	}
	source, err := NewVaultKVKeySource(server.URL, tokenFile, "secret", "app/key", server.Client(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.ActiveSigningKey(t.Context()); err == nil {
		t.Fatal("malformed Vault key material was accepted")
	}
}
