package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	cloudflareAPI            = "https://api.cloudflare.com/client/v4"
	cloudflareRequestTimeout = 20 * time.Second

	cfPhaseCacheSettings = "http_request_cache_settings"
	cfPhaseOrigin        = "http_request_origin"
	cfPhaseRateLimit     = "http_ratelimit"

	cfKindZone = "zone"
	cfAutoTTL  = 1 // Cloudflare: 1 表示 Auto，开代理时 Cloudflare 通常要求 Auto TTL
)

const cfCacheExtensionsExpression = `(http.request.uri.path.extension in {"html" "ts" "7z" "avi" "avif" "apk" "bin" "bmp" "bz2" "class" "css" "csv" "doc" "docx" "dmg" "ejs" "eot" "eps" "exe" "flac" "gif" "gz" "ico" "iso" "jar" "jpg" "jpeg" "js" "mid" "midi" "mkv" "mp3" "mp4" "ogg" "otf" "pdf" "pict" "pls" "png" "ppt" "pptx" "ps" "rar" "svg" "svgz" "swf" "tar" "tif" "tiff" "ttf" "webm" "webp" "woff" "woff2" "xls" "xlsx" "zip" "zst"})`

var cfHTTPClient = &http.Client{Timeout: cloudflareRequestTimeout}

type cfAPIError struct {
	StatusCode int
	Errors     []interface{}
	Body       string
}

func (e *cfAPIError) Error() string {
	return fmt.Sprintf("cloudflare api failed: status=%d errors=%v body=%s", e.StatusCode, e.Errors, e.Body)
}

type cFResponse struct {
	Success bool            `json:"success"`
	Errors  []interface{}   `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

type cFZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cFDNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

type cFRule struct {
	ID          string `json:"id"`
	Ref         string `json:"ref"`
	Description string `json:"description"`
	Expression  string `json:"expression"`
	Action      string `json:"action"`
}

type cFRuleset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Kind        string   `json:"kind"`
	Phase       string   `json:"phase"`
	Rules       []cFRule `json:"rules"`
}

func cfRequest(token, method, reqURL string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("cloudflare request marshal failed: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, reqURL, reader)
	if err != nil {
		return fmt.Errorf("cloudflare request create failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cfHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cloudflare response read failed: %w", err)
	}

	var cfResp cFResponse
	if err := json.Unmarshal(data, &cfResp); err != nil {
		return fmt.Errorf("cloudflare response parse failed: %w, body=%s", err, string(data))
	}
	if resp.StatusCode >= http.StatusBadRequest || !cfResp.Success {
		return &cfAPIError{StatusCode: resp.StatusCode, Errors: cfResp.Errors, Body: string(data)}
	}
	if out == nil {
		return nil
	}
	if len(cfResp.Result) == 0 || string(cfResp.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(cfResp.Result, out); err != nil {
		return fmt.Errorf("cloudflare result parse failed: %w", err)
	}
	return nil
}

func cfURL(path string, values url.Values) string {
	reqURL := cloudflareAPI + path
	if len(values) > 0 {
		reqURL += "?" + values.Encode()
	}
	return reqURL
}

func cfIsNotFound(err error) bool {
	var apiErr *cfAPIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func cfNormalizeDomain(domain string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
}

func cfRecordName(rootDomain, subDomain string) string {
	rootDomain = cfNormalizeDomain(rootDomain)
	subDomain = strings.TrimSuffix(strings.TrimSpace(subDomain), ".")
	if subDomain == "" || subDomain == "@" || strings.EqualFold(subDomain, rootDomain) {
		return rootDomain
	}
	return subDomain + "." + rootDomain
}

func cfSubdomainHost(rootDomain, subDomain string) string {
	return cfRecordName(rootDomain, subDomain)
}

func cfRuleRef(prefix, name string) string {
	name = strings.Trim(strings.TrimSpace(name), ".")
	if name == "" || name == "@" {
		name = "root"
	}
	name = strings.NewReplacer(".", "_", "*", "wildcard", "-", "_").Replace(name)
	return prefix + name
}

func getCFZoneID(token, domain string) (string, error) {
	domain = cfNormalizeDomain(domain)
	if domain == "" {
		return "", fmt.Errorf("cloudflare zone domain is empty")
	}

	reqURL := cfURL("/zones", url.Values{"name": {domain}})
	var zones []cFZone
	if err := cfRequest(token, http.MethodGet, reqURL, nil, &zones); err != nil {
		return "", err
	}
	if len(zones) == 0 {
		return "", fmt.Errorf("cloudflare zone not found: %s", domain)
	}
	return zones[0].ID, nil
}

func findCFRecord(token, zoneID, recordName, recordType string) (*cFDNSRecord, error) {
	reqURL := cfURL(fmt.Sprintf("/zones/%s/dns_records", zoneID), url.Values{
		"name": {cfNormalizeDomain(recordName)},
		"type": {strings.ToUpper(strings.TrimSpace(recordType))},
	})

	var records []cFDNSRecord
	if err := cfRequest(token, http.MethodGet, reqURL, nil, &records); err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

func findCFRulesetByPhase(token, zoneID, phase string) (*cFRuleset, error) {
	reqURL := cfURL(fmt.Sprintf("/zones/%s/rulesets", zoneID), nil)
	var rulesets []cFRuleset
	if err := cfRequest(token, http.MethodGet, reqURL, nil, &rulesets); err != nil {
		return nil, err
	}
	for i := range rulesets {
		if rulesets[i].Phase == phase {
			return &rulesets[i], nil
		}
	}
	return nil, nil
}

func getCFEntrypointRuleset(token, zoneID, phase string) (*cFRuleset, error) {
	reqURL := cfURL(fmt.Sprintf("/zones/%s/rulesets/phases/%s/entrypoint", zoneID, phase), nil)
	var ruleset cFRuleset
	if err := cfRequest(token, http.MethodGet, reqURL, nil, &ruleset); err != nil {
		if cfIsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if ruleset.ID == "" {
		return nil, nil
	}
	return &ruleset, nil
}

func createCFRuleset(token, zoneID string, body map[string]any, out any) error {
	reqURL := cfURL(fmt.Sprintf("/zones/%s/rulesets", zoneID), nil)
	return cfRequest(token, http.MethodPost, reqURL, body, out)
}

func upsertCFRulesetByPhase(token, zoneID, phase, existsLog, createLog string, body map[string]any) error {
	ruleset, err := findCFRulesetByPhase(token, zoneID, phase)
	if err != nil {
		return err
	}
	if ruleset == nil {
		fmt.Println(createLog)
		return createCFRuleset(token, zoneID, body, nil)
	}

	fmt.Printf("%s: %s\n", existsLog, ruleset.ID)
	reqURL := cfURL(fmt.Sprintf("/zones/%s/rulesets/%s", zoneID, ruleset.ID), nil)
	return cfRequest(token, http.MethodPut, reqURL, body, nil)
}

func createCFOriginEntrypointRuleset(token, zoneID string) (*cFRuleset, error) {
	body := map[string]any{
		"name":  "Origin Rules",
		"kind":  cfKindZone,
		"phase": cfPhaseOrigin,
		"rules": []any{},
	}

	var ruleset cFRuleset
	if err := createCFRuleset(token, zoneID, body, &ruleset); err != nil {
		return nil, err
	}
	return &ruleset, nil
}

func findRuleByRef(ruleset *cFRuleset, ref string) *cFRule {
	if ruleset == nil || ref == "" {
		return nil
	}
	for i := range ruleset.Rules {
		if ruleset.Rules[i].Ref == ref {
			return &ruleset.Rules[i]
		}
	}
	return nil
}

func createCFRule(token, zoneID, rulesetID string, rule map[string]any) error {
	reqURL := cfURL(fmt.Sprintf("/zones/%s/rulesets/%s/rules", zoneID, rulesetID), nil)
	return cfRequest(token, http.MethodPost, reqURL, rule, nil)
}

func updateCFRule(token, zoneID, rulesetID, ruleID string, rule map[string]any) error {
	reqURL := cfURL(fmt.Sprintf("/zones/%s/rulesets/%s/rules/%s", zoneID, rulesetID, ruleID), nil)
	return cfRequest(token, http.MethodPatch, reqURL, rule, nil)
}

func upsertCFOriginRule(token, zoneID string, rule map[string]any) error {
	ref, _ := rule["ref"].(string)
	if ref == "" {
		return fmt.Errorf("cloudflare origin rule ref is empty")
	}

	ruleset, err := getCFEntrypointRuleset(token, zoneID, cfPhaseOrigin)
	if err != nil {
		return err
	}
	if ruleset == nil {
		ruleset, err = createCFOriginEntrypointRuleset(token, zoneID)
		if err != nil {
			return err
		}
	}

	existingRule := findRuleByRef(ruleset, ref)
	if existingRule == nil {
		fmt.Println("Cloudflare Origin Rule 不存在，创建:", ref)
		return createCFRule(token, zoneID, ruleset.ID, rule)
	}
	if existingRule.ID == "" {
		return fmt.Errorf("cloudflare origin rule id is empty: %s", ref)
	}

	fmt.Println("Cloudflare Origin Rule 已存在，更新:", ref)
	return updateCFRule(token, zoneID, ruleset.ID, existingRule.ID, rule)
}

func SetCFOriginPortForSubdomain(token, rootDomain, subDomain string, port int32) error {
	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return err
	}
	if port <= 0 {
		return fmt.Errorf("cloudflare origin port is invalid: %d", port)
	}

	hostName := cfSubdomainHost(rootDomain, subDomain)
	rule := map[string]any{
		"ref":         cfRuleRef("set_origin_port_", fmt.Sprintf("%d_%s", port, rootDomain)),
		"description": fmt.Sprintf("Set origin port %d for %s", port, rootDomain),
		"expression":  fmt.Sprintf(`http.host eq "%s"`, hostName),
		"action":      "route",
		"action_parameters": map[string]any{
			"origin": map[string]any{
				"port": port,
			},
		},
	}
	return upsertCFOriginRule(token, zoneID, rule)
}

func SetCFOriginHostHeaderForSubdomain(token, rootDomain, subDomain, originHost string) error {
	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return err
	}
	originHost = cfNormalizeDomain(originHost)
	if originHost == "" {
		return fmt.Errorf("cloudflare origin host header is empty")
	}

	hostName := cfSubdomainHost(rootDomain, subDomain)
	rule := map[string]any{
		"ref":         cfRuleRef("set_origin_host_header_", subDomain),
		"description": "Set origin Host header for " + hostName,
		"expression":  fmt.Sprintf(`http.host eq "%s"`, hostName),
		"action":      "route",
		"action_parameters": map[string]any{
			"host_header": originHost,
		},
	}
	return upsertCFOriginRule(token, zoneID, rule)
}

func SetCFCacheRule(token, rootDomain string) error {
	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return err
	}

	body := map[string]any{
		"name":        "cache selected extensions only",
		"description": "Cache only selected file extensions, bypass everything else",
		"kind":        cfKindZone,
		"phase":       cfPhaseCacheSettings,
		"rules": []map[string]any{
			{
				"ref":         "cache_selected_extensions",
				"description": "Cache selected extensions",
				"expression":  cfCacheExtensionsExpression,
				"action":      "set_cache_settings",
				"action_parameters": map[string]any{
					"cache": true,
					"edge_ttl": map[string]any{
						"mode":    "override_origin",
						"default": 86400,
					},
				},
			},
			{
				"ref":         "bypass_other_extensions",
				"description": "Bypass cache for everything else",
				"expression":  "true",
				"action":      "set_cache_settings",
				"action_parameters": map[string]any{
					"cache": false,
				},
			},
		},
	}

	return upsertCFRulesetByPhase(
		token,
		zoneID,
		cfPhaseCacheSettings,
		"Cloudflare 缓存规则已存在，更新 ruleset",
		"Cloudflare 缓存规则不存在，创建",
		body,
	)
}

func SetCFRateLimitRule(token, rootDomain string) error {
	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return err
	}

	body := map[string]any{
		"name":        "block ip over 1000 requests per 10 seconds",
		"description": "Block a single IP when it exceeds 1000 requests in 10 seconds",
		"kind":        cfKindZone,
		"phase":       cfPhaseRateLimit,
		"rules": []map[string]any{
			{
				"ref":         "block_ip_over_1000_requests_per_10_seconds",
				"description": "Block IPs exceeding 1000 requests in 10 seconds",
				"expression":  "true",
				"action":      "block",
				"ratelimit": map[string]any{
					"characteristics":     []string{"cf.colo.id", "ip.src"},
					"requests_to_origin":  false,
					"requests_per_period": 1000,
					"period":              10,
					"mitigation_timeout":  10,
				},
			},
		},
	}

	return upsertCFRulesetByPhase(
		token,
		zoneID,
		cfPhaseRateLimit,
		"Cloudflare 速率限制规则已存在，更新 ruleset",
		"Cloudflare 速率限制规则不存在，创建",
		body,
	)
}

func SetCFSSLMode(token, rootDomain, mode string) error {
	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return err
	}
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return fmt.Errorf("cloudflare ssl mode is empty")
	}

	reqURL := cfURL(fmt.Sprintf("/zones/%s/settings/ssl", zoneID), nil)
	return cfRequest(token, http.MethodPatch, reqURL, map[string]any{"value": mode}, nil)
}

func UpsertCFRecord(token, rootDomain, subDomain, recordType, value string, ttl int64, proxied bool) (bool, error) {
	rootDomain = cfNormalizeDomain(rootDomain)
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	value = strings.TrimSpace(value)
	if ttl <= 0 || proxied {
		ttl = cfAutoTTL
	}
	if recordType == "" {
		return true, fmt.Errorf("cloudflare record type is empty")
	}
	if value == "" {
		return true, fmt.Errorf("cloudflare record value is empty")
	}

	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return true, err
	}

	recordName := cfRecordName(rootDomain, subDomain)
	record, err := findCFRecord(token, zoneID, recordName, recordType)
	if err != nil {
		return true, err
	}

	body := map[string]any{
		"type":    recordType,
		"name":    recordName,
		"content": value,
		"ttl":     ttl,
		"proxied": proxied,
	}
	if record == nil {
		fmt.Printf("Cloudflare 记录不存在，创建: %s %s -> %s\n", recordName, recordType, value)
		reqURL := cfURL(fmt.Sprintf("/zones/%s/dns_records", zoneID), nil)
		return true, cfRequest(token, http.MethodPost, reqURL, body, nil)
	}
	if record.Content == value && record.Proxied == proxied && int64(record.TTL) == ttl {
		fmt.Printf("Cloudflare 记录未变化，跳过: %s -> %s\n", recordName, value)
		return false, nil
	}

	fmt.Printf("Cloudflare 记录存在，更新: %s %s -> %s\n", recordName, record.Content, value)
	reqURL := cfURL(fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, record.ID), nil)
	return true, cfRequest(token, http.MethodPut, reqURL, body, nil)
}
