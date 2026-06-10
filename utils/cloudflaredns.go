package utils

import (
	"bytes"
	"encoding/json"
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

	cfAutoTTL = 1 // Cloudflare: 1 表示 Auto，开代理时 Cloudflare 通常要求 Auto TTL
)

var cfHTTPClient = &http.Client{Timeout: cloudflareRequestTimeout}

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

type cFRuleset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Phase       string `json:"phase"`
}
type cFRule struct {
	ID          string `json:"id"`
	Ref         string `json:"ref"`
	Description string `json:"description"`
	Expression  string `json:"expression"`
	Action      string `json:"action"`
}

type cFOriginRuleset struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Kind  string   `json:"kind"`
	Phase string   `json:"phase"`
	Rules []cFRule `json:"rules"`
}

func cfRequest(token, method, url string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("cloudflare request marshal failed: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reader)
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

	var r cFResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("cloudflare response parse failed: %w, body=%s", err, string(data))
	}

	if !r.Success {
		return fmt.Errorf("cloudflare api failed: status=%d errors=%v body=%s", resp.StatusCode, r.Errors, string(data))
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(r.Result, out); err != nil {
		return fmt.Errorf("cloudflare result parse failed: %w", err)
	}
	return nil
}

func cfURL(path string, values url.Values) string {
	u := cloudflareAPI + path
	if len(values) > 0 {
		u += "?" + values.Encode()
	}
	return u
}

func getCFZoneID(token, domain string) (string, error) {
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
		"name": {recordName},
		"type": {recordType},
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

	for _, rs := range rulesets {
		if rs.Phase == phase {
			return &rs, nil
		}
	}

	return nil, nil
}

func createCFRuleset(token, zoneID string, body map[string]any) error {
	reqURL := cfURL(fmt.Sprintf("/zones/%s/rulesets", zoneID), nil)
	return cfRequest(token, http.MethodPost, reqURL, body, nil)
}

func upsertCFRulesetByPhase(token, zoneID, phase, existsLog, createLog string, body map[string]any) error {
	ruleset, err := findCFRulesetByPhase(token, zoneID, phase)
	if err != nil {
		return err
	}

	if ruleset != nil {
		fmt.Printf("%s: %s\n", existsLog, ruleset.ID)
		reqURL := cfURL(fmt.Sprintf("/zones/%s/rulesets/%s", zoneID, ruleset.ID), nil)
		return cfRequest(token, http.MethodPut, reqURL, body, nil)
	}

	fmt.Println(createLog)
	return createCFRuleset(token, zoneID, body)
}

func cfSubdomainHost(rootDomain, subDomain string) string {
	return strings.TrimSuffix(subDomain+"."+rootDomain, ".")
}

func cfRuleRef(prefix, name string) string {
	return prefix + strings.ReplaceAll(name, ".", "_")
}

func getCFOriginEntrypointRuleset(token, zoneID string) (*cFOriginRuleset, error) {
	reqURL := cfURL(
		fmt.Sprintf(
			"/zones/%s/rulesets/phases/%s/entrypoint", zoneID, cfPhaseOrigin,
		),
		nil,
	)

	var rs cFOriginRuleset

	err := cfRequest(
		token,
		http.MethodGet,
		reqURL,
		nil,
		&rs,
	)

	if err != nil {
		return nil, nil
	}

	if rs.ID == "" {
		return nil, nil
	}

	return &rs, nil
}
func createCFOriginEntrypointRuleset(token, zoneID string) (*cFOriginRuleset, error) {
	body := map[string]any{
		"name":  "Origin Rules",
		"kind":  "zone",
		"phase": cfPhaseOrigin,
		"rules": []any{},
	}

	reqURL := cfURL(
		fmt.Sprintf(
			"/zones/%s/rulesets",
			zoneID,
		),
		nil,
	)

	var rs cFOriginRuleset

	err := cfRequest(
		token,
		http.MethodPost,
		reqURL,
		body,
		&rs,
	)

	if err != nil {
		return nil, err
	}

	return &rs, nil
}
func createOriginRule(token, zoneID, rulesetID string, rule map[string]any) error {
	reqURL := cfURL(
		fmt.Sprintf(
			"/zones/%s/rulesets/%s/rules",
			zoneID,
			rulesetID,
		),
		nil,
	)

	return cfRequest(
		token,
		http.MethodPost,
		reqURL,
		rule,
		nil,
	)
}
func findRuleByRef(
	rs *cFOriginRuleset,
	ref string,
) *cFRule {

	for i := range rs.Rules {
		if rs.Rules[i].Ref == ref {
			return &rs.Rules[i]
		}
	}

	return nil
}
func updateOriginRule(
	token,
	zoneID,
	rulesetID,
	ruleID string,
	rule map[string]any,
) error {

	reqURL := cfURL(
		fmt.Sprintf(
			"/zones/%s/rulesets/%s/rules/%s",
			zoneID,
			rulesetID,
			ruleID,
		),
		nil,
	)

	return cfRequest(
		token,
		http.MethodPut,
		reqURL,
		rule,
		nil,
	)
}
func UpsertCFOriginRule(
	token,
	zoneID string,
	rule map[string]any,
) error {

	ref, _ := rule["ref"].(string)

	rs, err := getCFOriginEntrypointRuleset(
		token,
		zoneID,
	)

	if err != nil {
		return err
	}

	if rs == nil {

		rs, err = createCFOriginEntrypointRuleset(
			token,
			zoneID,
		)

		if err != nil {
			return err
		}
	}

	exists := findRuleByRef(
		rs,
		ref,
	)

	if exists == nil {

		fmt.Println(
			"Cloudflare Origin Rule 不存在，创建:",
			ref,
		)

		return createOriginRule(
			token,
			zoneID,
			rs.ID,
			rule,
		)
	}

	fmt.Println(
		"Cloudflare Origin Rule 已存在，更新:",
		ref,
	)

	return updateOriginRule(
		token,
		zoneID,
		rs.ID,
		exists.ID,
		rule,
	)
}
func SetCFOriginPortForSubdomain(
	token,
	rootDomain,
	subDomain string,
	port int32,
) error {

	zoneID, err := getCFZoneID(
		token,
		rootDomain,
	)

	if err != nil {
		return err
	}

	hostName := cfSubdomainHost(
		rootDomain,
		subDomain,
	)

	rule := map[string]any{
		"ref": cfRuleRef(
			"set_origin_port_",
			subDomain,
		),
		"description": fmt.Sprintf(
			"Set origin port %d for %s",
			port,
			hostName,
		),
		"expression": fmt.Sprintf(
			`http.host eq "%s"`,
			hostName,
		),
		"action": "route",
		"action_parameters": map[string]any{
			"origin": map[string]any{
				"port": port,
			},
		},
	}

	return UpsertCFOriginRule(
		token,
		zoneID,
		rule,
	)
}
func SetCFOriginHostHeaderForSubdomain(token, rootDomain, subDomain, originHost string) error {
	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return err
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

	return UpsertCFOriginRule(
		token,
		zoneID,
		rule,
	)
}

func SetCFCacheRule(token, rootDomain string) error {
	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return err
	}

	body := map[string]any{
		"name":        "cache selected extensions only",
		"description": "Cache only selected file extensions, bypass everything else",
		"kind":        "zone",
		"phase":       cfPhaseCacheSettings,
		"rules": []map[string]any{
			{
				"ref":         "cache_selected_extensions",
				"description": "Cache selected extensions",
				"expression":  `(http.request.uri.path.extension in {"html" "ts" "7z" "avi" "avif" "apk" "bin" "bmp" "bz2" "class" "css" "csv" "doc" "docx" "dmg" "ejs" "eot" "eps" "exe" "flac" "gif" "gz" "ico" "iso" "jar" "jpg" "jpeg" "js" "mid" "midi" "mkv" "mp3" "mp4" "ogg" "otf" "pdf" "pict" "pls" "png" "ppt" "pptx" "ps" "rar" "svg" "svgz" "swf" "tar" "tif" "tiff" "ttf" "webm" "webp" "woff" "woff2" "xls" "xlsx" "zip" "zst"})`,
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
		"kind":        "zone",
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

	reqURL := cfURL(fmt.Sprintf("/zones/%s/settings/ssl", zoneID), nil)
	return cfRequest(token, http.MethodPatch, reqURL, map[string]any{"value": mode}, nil)
}

func UpsertCFRecord(token, rootDomain, subDomain, recordType, value string, ttl int64, proxied bool) (bool, error) {
	if ttl <= 0 || proxied {
		ttl = cfAutoTTL
	}

	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return true, err
	}

	recordName := rootDomain
	if subDomain != "@" && subDomain != "" {
		recordName = subDomain + "." + rootDomain
	}
	recordName = strings.TrimSuffix(recordName, ".")

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

	if record.Content == value && record.Proxied == proxied {
		fmt.Printf("Cloudflare 记录未变化，跳过: %s -> %s\n", recordName, value)
		return false, nil
	}

	fmt.Printf("Cloudflare 记录存在，更新: %s %s -> %s\n", recordName, record.Content, value)
	reqURL := cfURL(fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, record.ID), nil)
	return true, cfRequest(token, http.MethodPut, reqURL, body, nil)
}
