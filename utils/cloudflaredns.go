package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const cloudflareAPI = "https://api.cloudflare.com/client/v4"

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

func cfRequest(token, method, url string, body any, out any) error {
	var reader io.Reader

	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	var r cFResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("cloudflare response parse failed: %w, body=%s", err, string(data))
	}

	if !r.Success {
		return fmt.Errorf("cloudflare api failed: status=%d errors=%v body=%s", resp.StatusCode, r.Errors, string(data))
	}

	if out != nil {
		if err := json.Unmarshal(r.Result, out); err != nil {
			return err
		}
	}
	return nil
}

func getCFZoneID(token, domain string) (string, error) {
	url := fmt.Sprintf("%s/zones?name=%s", cloudflareAPI, domain)

	var zones []cFZone
	if err := cfRequest(token, http.MethodGet, url, nil, &zones); err != nil {
		return "", err
	}

	if len(zones) == 0 {
		return "", fmt.Errorf("cloudflare zone not found: %s", domain)
	}

	return zones[0].ID, nil
}

func findCFRecord(token, zoneID, recordName, recordType string) (*cFDNSRecord, error) {
	url := fmt.Sprintf(
		"%s/zones/%s/dns_records?type=%s&name=%s",
		cloudflareAPI,
		zoneID,
		recordType,
		recordName,
	)

	var records []cFDNSRecord
	if err := cfRequest(token, http.MethodGet, url, nil, &records); err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, nil
	}

	return &records[0], nil
}
func findCFCacheRuleset(token, zoneID string) (*cFRuleset, error) {
	url := fmt.Sprintf("%s/zones/%s/rulesets", cloudflareAPI, zoneID)

	var rulesets []cFRuleset
	if err := cfRequest(token, http.MethodGet, url, nil, &rulesets); err != nil {
		return nil, err
	}

	for _, rs := range rulesets {
		if rs.Phase == "http_request_cache_settings" {
			return &rs, nil
		}
	}

	return nil, nil
}
func SetCFCacheRule(token, rootDomain string) error {
	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return err
	}

	ruleset, err := findCFCacheRuleset(token, zoneID)
	if err != nil {
		return err
	}

	expression := `(http.request.uri.path.extension in {"html" "ts" "7z" "avi" "avif" "apk" "bin" "bmp" "bz2" "class" "css" "csv" "doc" "docx" "dmg" "ejs" "eot" "eps" "exe" "flac" "gif" "gz" "ico" "iso" "jar" "jpg" "jpeg" "js" "mid" "midi" "mkv" "mp3" "mp4" "ogg" "otf" "pdf" "pict" "pls" "png" "ppt" "pptx" "ps" "rar" "svg" "svgz" "swf" "tar" "tif" "tiff" "ttf" "webm" "webp" "woff" "woff2" "xls" "xlsx" "zip" "zst"})`

	body := map[string]any{
		"name":        "cache selected extensions only",
		"description": "Cache only selected file extensions, bypass everything else",
		"kind":        "zone",
		"phase":       "http_request_cache_settings",
		"rules": []map[string]any{
			{
				"ref":         "cache_selected_extensions",
				"description": "Cache selected extensions",
				"expression":  expression,
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

	if ruleset != nil {
		fmt.Printf("Cloudflare 缓存规则已存在，更新 ruleset: %s\n", ruleset.ID)
		url := fmt.Sprintf("%s/zones/%s/rulesets/%s", cloudflareAPI, zoneID, ruleset.ID)
		return cfRequest(token, http.MethodPut, url, body, nil)
	}

	fmt.Println("Cloudflare 缓存规则不存在，创建")
	url := fmt.Sprintf("%s/zones/%s/rulesets", cloudflareAPI, zoneID)
	return cfRequest(token, http.MethodPost, url, body, nil)
}

func SetCFSSLMode(token, rootDomain, mode string) error {
	zoneID, err := getCFZoneID(token, rootDomain)
	if err != nil {
		return err
	}

	body := map[string]any{
		"value": mode,
	}

	url := fmt.Sprintf("%s/zones/%s/settings/ssl", cloudflareAPI, zoneID)

	return cfRequest(token, http.MethodPatch, url, body, nil)
}
func UpsertCFRecord(token, rootDomain, subDomain, recordType, value string, ttl int64, proxied bool) (bool, error) {
	if ttl <= 0 || proxied {
		ttl = 1 // Cloudflare: 1 表示 Auto 开代理时 Cloudflare 通常要求 Auto TTL
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

		url := fmt.Sprintf("%s/zones/%s/dns_records", cloudflareAPI, zoneID)
		return true, cfRequest(token, http.MethodPost, url, body, nil)
	}
	if record.Content == value && record.Proxied == proxied {
		fmt.Printf("Cloudflare 记录未变化，跳过: %s -> %s\n", recordName, value)
		return false, nil
	}
	fmt.Printf("Cloudflare 记录存在，更新: %s %s -> %s\n", recordName, record.Content, value)
	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", cloudflareAPI, zoneID, record.ID)
	return true, cfRequest(token, http.MethodPut, url, body, nil)
}
