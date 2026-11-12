package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPaystackInitializeUsesKoboAndServerCredentials(t *testing.T) {
	var gotAuthorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		if r.Method != http.MethodPost || r.URL.Path != "/transaction/initialize" { t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path) }
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil { t.Fatal(err) }
		if body["amount"] != "6500000" || body["currency"] != "NGN" { t.Fatalf("wrong amount/currency payload: %#v", body) }
		if body["email"] != "billing@school.ng" || body["reference"] != "SUB-2026-001" { t.Fatalf("wrong transaction identity: %#v", body) }
		if body["callback_url"] != "https://app.stonezdigital.com.ng/billing/return" { t.Fatalf("callback URL missing: %#v", body) }
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"message":"ok","data":{"authorization_url":"https://checkout.paystack.com/example","access_code":"access-1","reference":"SUB-2026-001"}}`))
	}))
	defer server.Close()

	p, err := NewPaystackProvider("sk_test_do_not_log", server.Client())
	if err != nil { t.Fatal(err) }
	p.baseURL = server.URL
	got, err := p.Initialize(context.Background(), PaystackInitializeInput{
		Email:"billing@school.ng", AmountMinor:6500000, Reference:"SUB-2026-001",
		CallbackURL:"https://app.stonezdigital.com.ng/billing/return",
	})
	if err != nil { t.Fatal(err) }
	if got.AuthorizationURL != "https://checkout.paystack.com/example" || got.Reference != "SUB-2026-001" { t.Fatalf("unexpected initialize response: %#v", got) }
	if gotAuthorization != "Bearer sk_test_do_not_log" { t.Fatalf("secret authorization header not set correctly") }
}

func TestPaystackVerifyRequiresSuccessfulMatchingNGNTransaction(t *testing.T) {
	paidAt := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/transaction/verify/SUB-2026-001" { t.Fatalf("unexpected verification request: %s %s", r.Method, r.URL.Path) }
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":true,"message":"verified","data":map[string]interface{}{
				"status":"success","reference":"SUB-2026-001","amount":6500000,"currency":"NGN",
				"paid_at":paidAt.Format(time.RFC3339),"metadata":map[string]interface{}{"purpose":"school_subscription"},
			},
		})
	}))
	defer server.Close()
	p, err := NewPaystackProvider("sk_test_do_not_log", server.Client())
	if err != nil { t.Fatal(err) }
	p.baseURL = server.URL
	got, err := p.Verify("SUB-2026-001")
	if err != nil { t.Fatal(err) }
	if got.Provider != "paystack" || got.Status != "succeeded" || got.AmountMinor != 6500000 || got.Currency != "NGN" {
		t.Fatalf("unexpected verified payment: %#v", got)
	}
	if got.Amount != 65000 || !got.PaidAt.Equal(paidAt) { t.Fatalf("amount or paid timestamp mismatch: %#v", got) }
}

func TestPaystackVerifyRejectsWrongCurrencyOrReference(t *testing.T) {
	cases := []struct{name, currency, reference string}{
		{"wrong currency","USD","SUB-2026-001"},
		{"wrong reference","NGN","OTHER-REF"},
	}
	for _, tc := range cases { t.Run(tc.name, func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status":true,"data":map[string]interface{}{
				"status":"success","reference":tc.reference,"amount":6500000,"currency":tc.currency,"paid_at":"2026-11-12T09:00:00Z",
			}})
		}))
		defer server.Close()
		p, err := NewPaystackProvider("sk_test_secret", server.Client()); if err != nil { t.Fatal(err) }; p.baseURL=server.URL
		if _, err = p.Verify("SUB-2026-001"); err == nil { t.Fatal("expected invalid transaction to be rejected") }
	}) }
}

func TestPaystackInitializeRejectsUnsafeInputs(t *testing.T) {
	p, err := NewPaystackProvider("sk_test_secret", nil); if err != nil { t.Fatal(err) }
	_, err = p.Initialize(context.Background(), PaystackInitializeInput{Email:"billing@school.ng",AmountMinor:6500000,Reference:"bad ref"})
	if err == nil { t.Fatal("expected invalid reference to be rejected") }
	_, err = p.Initialize(context.Background(), PaystackInitializeInput{Email:"billing@school.ng",AmountMinor:6500000,Reference:"SUB-1",CallbackURL:"http://insecure.example/return"})
	if err == nil { t.Fatal("expected non-HTTPS callback URL to be rejected") }
	if validPaystackReference(strings.Repeat("a", 101)) { t.Fatal("expected oversized reference to be rejected") }
}

func TestPaystackProviderRequiresSecretKey(t *testing.T) {
	if _, err := NewPaystackProvider(" ", nil); err != ErrPaystackNotConfigured { t.Fatalf("expected missing key error, got %v", err) }
}
