package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// CloudflareSchoolDomainAttachment is the exact attachment record returned by
// Cloudflare. DomainID is used only after hostname and Worker ownership match.
type CloudflareSchoolDomainAttachment struct {
	DomainID string
	Hostname string
	Service string
	Enabled bool
}

// LookupAttachment returns the exact hostname attachment, or nil when absent.
// A matching hostname attached to another Worker is returned to the caller so
// that it can be rejected rather than modified.
func (p *CloudflareSchoolDomainProvisioner) LookupAttachment(ctx context.Context, hostname string) (*CloudflareSchoolDomainAttachment, error) {
	if p == nil || p.token == "" || p.accountID == "" || p.workerName == "" {
		return nil, fmt.Errorf("Cloudflare school-domain provisioning is not configured")
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	if host == "" || !strings.HasSuffix(host, "."+p.baseDomain) {
		return nil, fmt.Errorf("hostname is outside the configured school base domain")
	}
	endpoint := strings.TrimRight(p.apiBaseURL, "/") + "/accounts/" + url.PathEscape(p.accountID) + "/workers/domains"
	query := url.Values{}
	query.Set("hostname", host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil { return nil, err }
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil { return nil, fmt.Errorf("Cloudflare attachment lookup failed: %w", err) }
	defer resp.Body.Close()
	var result struct {
		Success bool `json:"success"`
		Result []struct {
			ID string `json:"id"`
			Hostname string `json:"hostname"`
			Service string `json:"service"`
			Enabled bool `json:"enabled"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("Cloudflare returned an unreadable lookup response (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !result.Success {
		return nil, fmt.Errorf("Cloudflare attachment lookup failed (HTTP %d)", resp.StatusCode)
	}
	for _, item := range result.Result {
		if strings.EqualFold(item.Hostname, host) {
			return &CloudflareSchoolDomainAttachment{DomainID:item.ID, Hostname:item.Hostname, Service:item.Service, Enabled:item.Enabled}, nil
		}
	}
	return nil, nil
}

// DetachAttachment removes only the supplied attachment ID after verifying
// that its exact hostname is still attached to this configured Worker.
func (p *CloudflareSchoolDomainProvisioner) DetachAttachment(ctx context.Context, expected CloudflareSchoolDomainAttachment) error {
	if p == nil || p.token == "" || p.accountID == "" || p.workerName == "" {
		return fmt.Errorf("Cloudflare school-domain provisioning is not configured")
	}
	if expected.DomainID == "" || expected.Hostname == "" || expected.Service != p.workerName {
		return fmt.Errorf("refusing to detach an attachment without verified Worker ownership")
	}
	current, err := p.LookupAttachment(ctx, expected.Hostname)
	if err != nil { return err }
	if current == nil { return nil } // Idempotent retry after a prior successful detach.
	if current.DomainID != expected.DomainID || current.Service != p.workerName || !strings.EqualFold(current.Hostname, expected.Hostname) {
		return fmt.Errorf("Cloudflare attachment ownership changed; refusing to detach")
	}
	endpoint := strings.TrimRight(p.apiBaseURL, "/") + "/accounts/" + url.PathEscape(p.accountID) + "/workers/domains/" + url.PathEscape(expected.DomainID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil { return err }
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil { return fmt.Errorf("Cloudflare detach request failed: %w", err) }
	defer resp.Body.Close()
	var result struct { Success bool `json:"success"` }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("Cloudflare returned an unreadable detach response (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !result.Success {
		return fmt.Errorf("Cloudflare rejected domain detach (HTTP %d)", resp.StatusCode)
	}
	remaining, err := p.LookupAttachment(ctx, expected.Hostname)
	if err != nil { return fmt.Errorf("detach submitted but verification failed: %w", err) }
	if remaining != nil { return fmt.Errorf("Cloudflare still reports the hostname attached after detach") }
	return nil
}

// WorkerName returns the configured Worker service name for ownership comparisons.
func (p *CloudflareSchoolDomainProvisioner) WorkerName() string {
	if p == nil { return "" }
	return p.workerName
}
