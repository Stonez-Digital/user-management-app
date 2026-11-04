package server

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func mustCIDR(t *testing.T, value string) *net.IPNet {
	t.Helper()
	_, network, err := net.ParseCIDR(value)
	if err != nil {
		t.Fatal(err)
	}
	return network
}

func TestClientIPIgnoresForwardedHeaderFromUntrustedPeer(t *testing.T) {
	got := clientIPFromRequest("192.0.2.10:1234", "198.51.100.99", nil)
	if got != "192.0.2.10" {
		t.Fatalf("expected socket peer, got %q", got)
	}
}

func TestClientIPUsesForwardedChainOnlyFromTrustedPeer(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "10.0.0.0/8"), mustCIDR(t, "192.0.2.0/24")}
	got := clientIPFromRequest("10.0.0.5:443", "198.51.100.7, 192.0.2.8", trusted)
	if got != "198.51.100.7" {
		t.Fatalf("expected first untrusted hop from right, got %q", got)
	}
}

func TestClientIPDoesNotTrustSpoofedLeftmostValue(t *testing.T) {
	trusted := []*net.IPNet{mustCIDR(t, "10.0.0.0/8")}
	got := clientIPFromRequest("203.0.113.8:443", "198.51.100.99", trusted)
	if got != "203.0.113.8" {
		t.Fatalf("expected direct peer, got %q", got)
	}
}

func TestTrustedProxyNetworksRejectsInvalidCIDR(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "not-a-cidr")
	if _, err := trustedProxyNetworks(); err == nil {
		t.Fatal("expected invalid CIDR to be rejected")
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

func TestRateLimiterBoundsMemory(t *testing.T) {
	limiter := newRateLimiter()
	now := time.Now()
	for i := 0; i < maxRateLimitEntries; i++ {
		limiter.allow(fmt.Sprintf("client-%d", i), now)
	}
	if ok, _ := limiter.allow("overflow-client", now); ok {
		t.Fatal("new key should be rejected when the limiter is full")
	}
	if got := len(limiter.entries); got != maxRateLimitEntries {
		t.Fatalf("expected bounded map size %d, got %d", maxRateLimitEntries, got)
	}
}
