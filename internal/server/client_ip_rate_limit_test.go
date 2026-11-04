package server

import (
	"fmt"
	"testing"
	"time"
)

func TestClientIPUsesCloudflareConnectingIP(t *testing.T) {
	got := clientIPFromRequest("10.0.0.5:443", "198.51.100.7")
	if got != "198.51.100.7" {
		t.Fatalf("expected Cloudflare visitor IP, got %q", got)
	}
}

func TestClientIPFallsBackToSocketPeerForInvalidCloudflareHeader(t *testing.T) {
	got := clientIPFromRequest("192.0.2.10:1234", "not-an-ip")
	if got != "192.0.2.10" {
		t.Fatalf("expected socket peer fallback, got %q", got)
	}
}

func TestClientIPFallsBackToSocketPeerWhenCloudflareHeaderMissing(t *testing.T) {
	got := clientIPFromRequest("[2001:db8::10]:443", "")
	if got != "2001:db8::10" {
		t.Fatalf("expected IPv6 socket peer, got %q", got)
	}
}

func TestRateLimiterSeparatesClientsAndEnforcesLimit(t *testing.T) {
	limiter := newRateLimiter()
	now := time.Now()
	for i := 0; i < authRateLimit; i++ {
		if ok, _ := limiter.allow("198.51.100.1", now); !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if ok, _ := limiter.allow("198.51.100.1", now); ok {
		t.Fatal("expected the 11th request to be rejected")
	}
	if ok, _ := limiter.allow("198.51.100.2", now); !ok {
		t.Fatal("a different client should have an independent bucket")
	}
}

func TestRateLimiterExpiresAndCleansOldEntries(t *testing.T) {
	limiter := newRateLimiter()
	now := time.Now()
	limiter.allow("expired-client", now)
	for i := 0; i < cleanupEvery; i++ {
		limiter.allow(fmt.Sprintf("client-%d", i), now.Add(authRateWindow))
	}
	if _, ok := limiter.entries["expired-client"]; ok {
		t.Fatal("expired entry should have been cleaned up")
	}
}

func TestRateLimiterBoundsMemoryAndUsesOverflowBucket(t *testing.T) {
	limiter := newRateLimiter()
	now := time.Now()
	for i := 0; i < maxRateLimitEntries; i++ {
		limiter.allow(fmt.Sprintf("client-%d", i), now)
	}
	if ok, _ := limiter.allow("overflow-client-1", now); !ok {
		t.Fatal("first overflow request should use the shared overflow bucket")
	}
	for i := 1; i < authRateLimit; i++ {
		if ok, _ := limiter.allow(fmt.Sprintf("overflow-client-%d", i+1), now); !ok {
			t.Fatalf("overflow request %d should be allowed", i+1)
		}
	}
	if ok, _ := limiter.allow("overflow-client-11", now); ok {
		t.Fatal("overflow bucket should enforce the shared request limit")
	}
	if got := len(limiter.entries); got != maxRateLimitEntries {
		t.Fatalf("expected bounded map size %d, got %d", maxRateLimitEntries, got)
	}
	if ok, _ := limiter.allow("client-0", now); !ok {
		t.Fatal("existing client should not be blocked by overflow traffic")
	}
}

func TestRateLimiterReclaimsExpiredEntriesWhenFull(t *testing.T) {
	limiter := newRateLimiter()
	now := time.Now()
	for i := 0; i < maxRateLimitEntries; i++ {
		limiter.allow(fmt.Sprintf("client-%d", i), now)
	}
	if ok, _ := limiter.allow("new-client", now.Add(authRateWindow)); !ok {
		t.Fatal("new client should be admitted after expired entries are reclaimed")
	}
	if got := len(limiter.entries); got != 1 {
		t.Fatalf("expected expired entries to be reclaimed, got %d entries", got)
	}
}
