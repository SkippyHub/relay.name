package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DNSProvider interface for managing DNS records
type DNSProvider interface {
	CreateRecord(subdomain, ip string) error
	UpdateRecord(subdomain, ip string) error
	DeleteRecord(subdomain string) error
	GetRecord(subdomain string) (string, error)
}

// CloudflareDNS implements DNSProvider for Cloudflare
type CloudflareDNS struct {
	APIKey  string
	ZoneID  string
	Email   string
	BaseURL string
	Domain  string
}

func NewCloudflareDNS(apiKey, zoneID, email, domain string) *CloudflareDNS {
	return &CloudflareDNS{
		APIKey:  apiKey,
		ZoneID:  zoneID,
		Email:   email,
		BaseURL: "https://api.cloudflare.com/client/v4",
		Domain:  domain,
	}
}

type CloudflareResponse struct {
	Success bool                  `json:"success"`
	Result  []CloudflareDNSRecord `json:"result"`
	Errors  []CloudflareError     `json:"errors"`
}

type CloudflareDNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

type CloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (cf *CloudflareDNS) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s/zones/%s/%s", cf.BaseURL, cf.ZoneID, endpoint)

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Auth-Email", cf.Email)
	req.Header.Set("X-Auth-Key", cf.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}

func (cf *CloudflareDNS) getRecordID(subdomain string) (string, error) {
	fullName := subdomain
	if subdomain != "@" && subdomain != "*" {
		fullName = fmt.Sprintf("%s.%s", subdomain, cf.Domain)
	}

	resp, err := cf.makeRequest("GET", fmt.Sprintf("dns_records?name=%s", fullName), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var cfResp CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return "", err
	}

	if !cfResp.Success {
		return "", fmt.Errorf("cloudflare API error: %v", cfResp.Errors)
	}

	if len(cfResp.Result) > 0 {
		return cfResp.Result[0].ID, nil
	}

	return "", nil // Record doesn't exist
}

func (cf *CloudflareDNS) CreateRecord(subdomain, ip string) error {
	fullName := subdomain
	if subdomain != "@" && subdomain != "*" {
		fullName = fmt.Sprintf("%s.%s", subdomain, cf.Domain)
	}

	data := map[string]interface{}{
		"type":    "A",
		"name":    fullName,
		"content": ip,
		"ttl":     300, // 5 minutes
	}

	resp, err := cf.makeRequest("POST", "dns_records", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var cfResp CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return err
	}

	if !cfResp.Success {
		return fmt.Errorf("failed to create DNS record: %v", cfResp.Errors)
	}

	return nil
}

func (cf *CloudflareDNS) UpdateRecord(subdomain, ip string) error {
	recordID, err := cf.getRecordID(subdomain)
	if err != nil {
		return err
	}

	if recordID == "" {
		// Record doesn't exist, create it
		return cf.CreateRecord(subdomain, ip)
	}

	fullName := subdomain
	if subdomain != "@" && subdomain != "*" {
		fullName = fmt.Sprintf("%s.%s", subdomain, cf.Domain)
	}

	data := map[string]interface{}{
		"type":    "A",
		"name":    fullName,
		"content": ip,
		"ttl":     300,
	}

	resp, err := cf.makeRequest("PUT", fmt.Sprintf("dns_records/%s", recordID), data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var cfResp CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return err
	}

	if !cfResp.Success {
		return fmt.Errorf("failed to update DNS record: %v", cfResp.Errors)
	}

	return nil
}

func (cf *CloudflareDNS) DeleteRecord(subdomain string) error {
	recordID, err := cf.getRecordID(subdomain)
	if err != nil {
		return err
	}

	if recordID == "" {
		return nil // Record doesn't exist, nothing to delete
	}

	resp, err := cf.makeRequest("DELETE", fmt.Sprintf("dns_records/%s", recordID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var cfResp CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return err
	}

	if !cfResp.Success {
		return fmt.Errorf("failed to delete DNS record: %v", cfResp.Errors)
	}

	return nil
}

func (cf *CloudflareDNS) GetRecord(subdomain string) (string, error) {
	fullName := subdomain
	if subdomain != "@" && subdomain != "*" {
		fullName = fmt.Sprintf("%s.%s", subdomain, cf.Domain)
	}

	resp, err := cf.makeRequest("GET", fmt.Sprintf("dns_records?name=%s", fullName), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var cfResp CloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&cfResp); err != nil {
		return "", err
	}

	if !cfResp.Success || len(cfResp.Result) == 0 {
		return "", fmt.Errorf("record not found")
	}

	return cfResp.Result[0].Content, nil
}

// DigitalOceanDNS implements DNSProvider for DigitalOcean
type DigitalOceanDNS struct {
	APIToken string
	Domain   string
	BaseURL  string
}

func NewDigitalOceanDNS(apiToken, domain string) *DigitalOceanDNS {
	return &DigitalOceanDNS{
		APIToken: apiToken,
		Domain:   domain,
		BaseURL:  "https://api.digitalocean.com/v2",
	}
}

type DOResponse struct {
	DomainRecord struct {
		ID   int    `json:"id"`
		Type string `json:"type"`
		Name string `json:"name"`
		Data string `json:"data"`
		TTL  int    `json:"ttl"`
	} `json:"domain_record"`
}

type DOListResponse struct {
	DomainRecords []struct {
		ID   int    `json:"id"`
		Type string `json:"type"`
		Name string `json:"name"`
		Data string `json:"data"`
	} `json:"domain_records"`
}

func (do *DigitalOceanDNS) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s/domains/%s/%s", do.BaseURL, do.Domain, endpoint)

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", do.APIToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	return client.Do(req)
}

func (do *DigitalOceanDNS) getRecordID(subdomain string) (int, error) {
	resp, err := do.makeRequest("GET", "records", nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var doResp DOListResponse
	if err := json.NewDecoder(resp.Body).Decode(&doResp); err != nil {
		return 0, err
	}

	for _, record := range doResp.DomainRecords {
		if record.Type == "A" && record.Name == subdomain {
			return record.ID, nil
		}
	}

	return 0, nil // Not found
}

func (do *DigitalOceanDNS) CreateRecord(subdomain, ip string) error {
	data := map[string]interface{}{
		"type": "A",
		"name": subdomain,
		"data": ip,
		"ttl":  300,
	}

	resp, err := do.makeRequest("POST", "records", data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create DNS record: %s", string(body))
	}

	return nil
}

func (do *DigitalOceanDNS) UpdateRecord(subdomain, ip string) error {
	recordID, err := do.getRecordID(subdomain)
	if err != nil {
		return err
	}

	if recordID == 0 {
		return do.CreateRecord(subdomain, ip)
	}

	data := map[string]interface{}{
		"data": ip,
		"ttl":  300,
	}

	resp, err := do.makeRequest("PUT", fmt.Sprintf("records/%d", recordID), data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update DNS record: %s", string(body))
	}

	return nil
}

func (do *DigitalOceanDNS) DeleteRecord(subdomain string) error {
	recordID, err := do.getRecordID(subdomain)
	if err != nil {
		return err
	}

	if recordID == 0 {
		return nil
	}

	resp, err := do.makeRequest("DELETE", fmt.Sprintf("records/%d", recordID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete DNS record: %s", string(body))
	}

	return nil
}

func (do *DigitalOceanDNS) GetRecord(subdomain string) (string, error) {
	resp, err := do.makeRequest("GET", "records", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var doResp DOListResponse
	if err := json.NewDecoder(resp.Body).Decode(&doResp); err != nil {
		return "", err
	}

	for _, record := range doResp.DomainRecords {
		if record.Type == "A" && record.Name == subdomain {
			return record.Data, nil
		}
	}

	return "", fmt.Errorf("record not found")
}
