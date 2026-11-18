package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func SchoolBaseDomainFromEnv() string {
	domain := strings.Trim(strings.ToLower(strings.TrimSpace(os.Getenv("SCHOOL_BASE_DOMAIN"))), ".")
	if domain == "" { return "stonezdigital.com.ng" }
	return domain
}

type CloudflareSchoolDomainProvisioner struct {
	token      string
	accountID  string
	workerName string
	baseDomain string
	apiBaseURL string
	client     *http.Client
}

func NewCloudflareSchoolDomainProvisionerFromEnv() *CloudflareSchoolDomainProvisioner {
	token := strings.TrimSpace(os.Getenv("CLOUDFLARE_API_TOKEN"))
	accountID := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
	workerName := strings.TrimSpace(os.Getenv("CLOUDFLARE_WORKER_NAME"))
	baseDomain := SchoolBaseDomainFromEnv()
	if token == "" || accountID == "" || workerName == "" {
		return nil
	}
	return &CloudflareSchoolDomainProvisioner{
		token: token, accountID: accountID, workerName: workerName, baseDomain: baseDomain,
		apiBaseURL: "https://api.cloudflare.com/client/v4", client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *CloudflareSchoolDomainProvisioner) Attach(ctx context.Context, hostname string) error {
	if p == nil || p.token == "" || p.accountID == "" || p.workerName == "" {
		return fmt.Errorf("Cloudflare school-domain provisioning is not configured")
	}
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	suffix := "." + p.baseDomain
	if !strings.HasSuffix(host, suffix) {
		return fmt.Errorf("school hostname is outside the configured base domain")
	}
	label := strings.TrimSuffix(host, suffix)
	if label == "" || strings.Contains(label, ".") || len(label) > 63 ||
		!validCloudflareLabel(label) || reservedCloudflareSchoolLabels[label] {
		return fmt.Errorf("invalid or reserved school hostname")
	}

	body, err := json.Marshal(map[string]string{"hostname": host, "service": p.workerName})
	if err != nil {
		return err
	}
	url := strings.TrimRight(p.apiBaseURL, "/") + "/accounts/" + p.accountID + "/workers/domains"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("Cloudflare domain request failed: %w", err)
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("Cloudflare returned an unreadable response (HTTP %d)", resp.StatusCode)
	}
	success, _ := result["success"].(bool)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !success {
		return fmt.Errorf("Cloudflare rejected the domain (HTTP %d)", resp.StatusCode)
	}
	return p.verifyAttached(ctx, host)
}

func (p *CloudflareSchoolDomainProvisioner) verifyAttached(ctx context.Context, hostname string) error {
	endpoint := strings.TrimRight(p.apiBaseURL, "/") + "/accounts/" + p.accountID + "/workers/domains"
	query := url.Values{}
	query.Set("hostname", hostname)
	query.Set("service", p.workerName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
	if err != nil { return err }
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil { return fmt.Errorf("Cloudflare domain verification request failed: %w", err) }
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("Cloudflare returned an unreadable verification response (HTTP %d)", resp.StatusCode)
	}
	success, _ := result["success"].(bool)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !success {
		return fmt.Errorf("Cloudflare domain verification failed (HTTP %d)", resp.StatusCode)
	}
	entries, _ := result["result"].([]interface{})
	for _, entry := range entries {
		domain, _ := entry.(map[string]interface{})
		host, _ := domain["hostname"].(string)
		service, _ := domain["service"].(string)
		enabled, _ := domain["enabled"].(bool)
		if strings.EqualFold(host, hostname) && service == p.workerName && enabled { return nil }
	}
	return fmt.Errorf("Cloudflare did not confirm an enabled domain attachment to the configured Worker")
}

// Keep this list aligned with controller.reservedSchoolSlugs so a slug accepted
// during school creation cannot later fail Cloudflare hostname validation.
var reservedCloudflareSchoolLabels = map[string]bool{
	"app": true, "api": true, "www": true, "mail": true, "smtp": true,
	"ftp": true, "admin": true, "autodiscover": true,
}

func validCloudflareLabel(label string) bool {
	if len(label) == 0 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, r := range label {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}


// CloudflareWorkerDomain is the read-only subset needed to audit Worker
// custom-domain attachments.
type CloudflareWorkerDomain struct {
	Hostname string `json:"hostname"`
	Service  string `json:"service"`
	Enabled  bool   `json:"enabled"`
}

type cloudflareWorkerDomainListResponse struct {
	Success bool                    `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result     []CloudflareWorkerDomain `json:"result"`
	ResultInfo struct {
		TotalPages int `json:"total_pages"`
	} `json:"result_info"`
}

// ListAttachedDomains returns Worker custom domains without changing Cloudflare
// state. It is intentionally separate from Attach so audit tooling can remain
// read-only.
func (p *CloudflareSchoolDomainProvisioner) ListAttachedDomains(ctx context.Context) ([]CloudflareWorkerDomain, error) {
	if p == nil || p.token == "" || p.accountID == "" || p.workerName == "" {
		return nil, fmt.Errorf("Cloudflare school-domain provisioning is not configured")
	}

	const perPage = 100
	var domains []CloudflareWorkerDomain
	for page := 1; ; page++ {
		endpoint := strings.TrimRight(p.apiBaseURL, "/") + "/accounts/" + p.accountID + "/workers/domains"
		query := url.Values{}
		query.Set("service", p.workerName)
		query.Set("per_page", strconv.Itoa(perPage))
		query.Set("page", strconv.Itoa(page))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+p.token)
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Cloudflare domain audit request failed: %w", err)
		}

		var result cloudflareWorkerDomainListResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("Cloudflare returned an unreadable domain audit response (HTTP %d)", resp.StatusCode)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || !result.Success {
			if len(result.Errors) > 0 {
				return nil, fmt.Errorf("Cloudflare domain audit failed (HTTP %d): %s", resp.StatusCode, result.Errors[0].Message)
			}
			return nil, fmt.Errorf("Cloudflare domain audit failed (HTTP %d)", resp.StatusCode)
		}

		domains = append(domains, result.Result...)
		if result.ResultInfo.TotalPages == 0 || page >= result.ResultInfo.TotalPages {
			break
		}
	}
	return domains, nil
}
