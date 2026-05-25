package utils

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const ucloudAPI = "https://api.ucloud.cn"

const (
	actionGetCDNVerifyTXT = "GetUcdnDomainVerifyInfo" // 如果报 Action 不存在，按你账号文档改这里
	actionVerifyCDNOwner  = "VerifyUcdnDomainOwner"   // 如果报 Action 不存在，按你账号文档改这里
	actionUpdateCDN       = "UpdateUcdnDomainConfig"  // 如果报 Action 不存在，按你账号文档改这里
)

type uResp map[string]any

var ucloudHTTPClient = &http.Client{Timeout: 30 * time.Second}

func uSign(params map[string]string, privateKey string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "Signature" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	b.WriteString(privateKey)

	sum := sha1.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func uCall(publicKey, privateKey, projectID, region string, params map[string]string) (uResp, error) {
	p := make(map[string]string, len(params)+4)
	for k, v := range params {
		p[k] = v
	}

	if region != "" {
		p["Region"] = region
	}

	if projectID != "" {
		p["ProjectId"] = projectID
	}

	p["PublicKey"] = publicKey
	p["Signature"] = uSign(p, privateKey)

	v := url.Values{}
	for k, val := range p {
		v.Set(k, val)
	}

	body := v.Encode()
	req, err := http.NewRequest("POST", ucloudAPI, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ucloudHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ucloud http failed: status=%s action=%s body=%s", resp.Status, p["Action"], string(data))
	}

	var out uResp
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("ucloud response parse failed: %w body=%s", err, string(data))
	}

	if ret, ok := ucloudRetCode(out["RetCode"]); ok && ret != 0 {
		return out, fmt.Errorf("ucloud api failed: action=%s ret=%v msg=%v body=%s",
			p["Action"], out["RetCode"], out["Message"], string(data))
	}

	return out, nil
}

func ucloudRetCode(v any) (int, bool) {
	switch ret := v.(type) {
	case nil:
		return 0, false
	case json.Number:
		i, err := strconv.Atoi(ret.String())
		if err == nil {
			return i, true
		}
		f, err := strconv.ParseFloat(ret.String(), 64)
		return int(f), err == nil
	case float64:
		return int(ret), true
	case string:
		if ret == "" {
			return 0, false
		}
		i, err := strconv.Atoi(ret)
		if err == nil {
			return i, true
		}
		f, err := strconv.ParseFloat(ret, 64)
		return int(f), err == nil
	default:
		return 0, false
	}
}
func getUCloudProjectID(publicKey, privateKey string) (string, error) {
	resp, err := uCall(publicKey, privateKey, "", "", map[string]string{
		"Action": "GetProjectList",
	})
	if err != nil {
		return "", err
	}

	for _, key := range []string{"ProjectSet", "Projects", "ProjectList", "DataSet", "Infos"} {
		arr, ok := resp[key].([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		for _, item := range arr {
			m, _ := item.(map[string]any)
			if isDefault, _ := m["IsDefault"].(bool); isDefault {
				if projectID := strAny(m["ProjectId"]); projectID != "" {
					return projectID, nil
				}
			}
		}
		for _, item := range arr {
			m, _ := item.(map[string]any)
			if projectID := strAny(m["ProjectId"]); projectID != "" {
				return projectID, nil
			}
		}
	}
	return "", fmt.Errorf("ucloud project id not found")
}
func getUCloudCDNVerifyTXT(dns DNSRecord) (rr, value string, err error) {
	resp, err := uCall(dns.Key, dns.Secret, dns.ProjectID, dns.Region, map[string]string{
		"Action": actionGetCDNVerifyTXT,
		"Domain": dns.Domain,
	})
	if err != nil {
		return "", "", err
	}

	rr = strAny(resp["DnsVerifyName"])
	if rr == "" {
		rr = strAny(resp["VerifyKey"])
	}
	if rr == "" {
		rr = "cdn-verification"
	}

	value = strAny(resp["VerifyContent"])
	if value == "" {
		value = strAny(resp["VerifyCode"])
	}
	if value == "" {
		return "", "", fmt.Errorf("UCloud CDN verify content empty: %#v", resp)
	}

	rr = strings.TrimSuffix(rr, "."+dns.RootDomain)
	rr = strings.TrimSuffix(rr, ".")
	return rr, value, nil
}

func normalizeUCloudDomain(domain string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
}

func normalizeUCloudRR(rootDomain, rr string) string {
	rootDomain = normalizeUCloudDomain(rootDomain)
	rr = strings.TrimSuffix(strings.TrimSpace(rr), ".")
	if rr == "" || rr == "@" {
		return "@"
	}
	if strings.EqualFold(rr, rootDomain) {
		return "@"
	}
	suffix := "." + rootDomain
	if rootDomain != "" && strings.HasSuffix(strings.ToLower(rr), suffix) {
		rr = rr[:len(rr)-len(suffix)]
		if rr == "" {
			return "@"
		}
	}
	return rr
}

func ucloudRRForDomain(rootDomain, domain, rr string) string {
	rootDomain = normalizeUCloudDomain(rootDomain)
	rr = normalizeUCloudRR(rootDomain, rr)
	if rr != "@" {
		return rr
	}
	return normalizeUCloudRR(rootDomain, domain)
}

func UpsertUCloudDNSRecord(publicKey, privateKey, rootDomain, rr, recordType, value, projectID, region string, ttl int64) (bool, error) {
	if ttl <= 0 {
		ttl = 600
	}
	rootDomain = normalizeUCloudDomain(rootDomain)
	rr = normalizeUCloudRR(rootDomain, rr)
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	value = strings.TrimSpace(value)
	if rootDomain == "" {
		return true, fmt.Errorf("UCloud DNS root domain is empty")
	}
	if recordType == "" {
		return true, fmt.Errorf("UCloud DNS record type is empty")
	}
	if value == "" {
		return true, fmt.Errorf("UCloud DNS record value is empty: %s %s", rr, recordType)
	}

	zoneID, err := getUCloudDNSZoneID(publicKey, privateKey, rootDomain, projectID, region)
	if err != nil {
		return true, err
	}

	old, err := findUCloudDNSRecord(publicKey, privateKey, zoneID, rr, recordType, projectID, region)
	if err != nil {
		return true, err
	}

	recordValue := value + "|1|1"

	if old.ID == "" {
		_, err := uCall(publicKey, privateKey, projectID, region, map[string]string{
			"Action":    "CreateUDNSRecord",
			"DNSZoneId": zoneID,
			"Name":      rr,
			"Type":      recordType,
			"Value":     recordValue,
			"ValueType": "Normal",
			"TTL":       fmt.Sprint(ttl),
		})
		return true, err
	}

	if old.Value == value {
		return false, nil
	}

	_, err = uCall(publicKey, privateKey, projectID, region, map[string]string{
		"Action":      "UpdateUDNSRecord",
		"DNSZoneId":   zoneID,
		"DNSRecordId": old.ID,
		"Name":        rr,
		"Type":        recordType,
		"Value":       recordValue,
		"ValueType":   "Normal",
		"TTL":         fmt.Sprint(ttl),
	})
	return true, err
}

type uDNSRecord struct {
	ID    string
	Value string
}

func getUCloudDNSZoneID(publicKey, privateKey, rootDomain, projectID, region string) (string, error) {
	rootDomain = normalizeUCloudDomain(rootDomain)
	resp, err := uCall(publicKey, privateKey, projectID, region, map[string]string{
		"Action": "DescribeUDNSZone",
	})
	if err != nil {
		return "", err
	}

	for _, key := range []string{"DataSet", "ZoneList"} {
		if arr, ok := resp[key].([]any); ok {
			for _, item := range arr {
				m, _ := item.(map[string]any)
				name := normalizeUCloudDomain(strAny(m["Name"]))
				if strings.EqualFold(name, rootDomain) {
					id := strAny(m["DNSZoneId"])
					if id == "" {
						id = strAny(m["ZoneId"])
					}
					if id != "" {
						return id, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("UCloud DNS zone not found: %s", rootDomain)
}

func findUCloudDNSRecord(publicKey, privateKey, zoneID, rr, recordType, projectID, region string) (uDNSRecord, error) {
	rr = normalizeUCloudRR("", rr)
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	resp, err := uCall(publicKey, privateKey, projectID, region, map[string]string{
		"Action":    "DescribeUDNSRecord",
		"DNSZoneId": zoneID,
	})
	if err != nil {
		return uDNSRecord{}, err
	}

	for _, key := range []string{"DataSet", "RecordList"} {
		if arr, ok := resp[key].([]any); ok {
			for _, item := range arr {
				m, _ := item.(map[string]any)
				if strAny(m["Name"]) == rr && strings.EqualFold(strAny(m["Type"]), recordType) {
					val := strAny(m["Value"])
					val = strings.Split(val, "|")[0]
					return uDNSRecord{
						ID:    firstNonEmpty(strAny(m["DNSRecordId"]), strAny(m["RecordId"])),
						Value: val,
					}, nil
				}
			}
		}
	}

	return uDNSRecord{}, nil
}

func verifyUCloudCDNOwner(publicKey, privateKey, domain, projectID, region string) error {
	_, err := uCall(publicKey, privateKey, projectID, region, map[string]string{
		"Action":     actionVerifyCDNOwner,
		"Domain":     domain,
		"VerifyType": "dns",
	})
	return err
}

type uCDNDomain struct {
	ID     string
	CNAME  string
	Exists bool
}

func getUCloudCDNDomain(publicKey, privateKey, domain, projectID, region string) (uCDNDomain, error) {
	domain = normalizeUCloudDomain(domain)
	if domain == "" {
		return uCDNDomain{}, fmt.Errorf("UCloud CDN domain is empty")
	}

	queries := []map[string]string{
		{
			"Action":   "GetUcdnDomainInfoList",
			"PageSize": "100",
		},
		{
			"Action":   "GetUcdnDomainInfoList",
			"PageSize": "100",
			"IsDcdn":   "true",
		},
		{
			"Action": "GetUcdnDomainConfig",
			"Limit":  "100",
		},
		{
			"Action": "GetUcdnDomainConfig",
			"Limit":  "100",
			"IsDcdn": "true",
		},
		{
			"Action":   "GetUcdnDomainConfig",
			"Domain.0": domain,
			"Limit":    "100",
		},
	}

	var lastErr error
	anySuccess := false
	for _, params := range queries {
		resp, err := uCall(publicKey, privateKey, projectID, region, params)
		if err != nil {
			lastErr = err
			continue
		}
		anySuccess = true
		if d, ok := findUCloudCDNDomainInResp(resp, domain); ok {
			return d, nil
		}
	}

	if !anySuccess && lastErr != nil {
		return uCDNDomain{}, lastErr
	}
	return uCDNDomain{}, nil
}

func findUCloudCDNDomainInResp(resp uResp, domain string) (uCDNDomain, bool) {
	for _, listKey := range []string{"DomainInfoList", "DomainConfigList", "DomainList", "DataSet", "Data"} {
		arr, ok := resp[listKey].([]any)
		if !ok {
			continue
		}
		if d, ok := findUCloudCDNDomainInList(arr, domain); ok {
			return d, true
		}
	}

	for _, v := range resp {
		arr, ok := v.([]any)
		if !ok {
			continue
		}
		if d, ok := findUCloudCDNDomainInList(arr, domain); ok {
			return d, true
		}
	}
	return uCDNDomain{}, false
}

func findUCloudCDNDomainInList(arr []any, domain string) (uCDNDomain, bool) {
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if d, ok := extractUCloudCDNDomain(m, domain); ok {
			return d, true
		}
	}
	return uCDNDomain{}, false
}

func extractUCloudCDNDomain(m map[string]any, domain string) (uCDNDomain, bool) {
	name := firstNonEmpty(
		strAny(m["Domain"]),
		strAny(m["DomainName"]),
		strAny(m["CdnDomain"]),
		strAny(m["CdnDomainName"]),
	)
	if !strings.EqualFold(normalizeUCloudDomain(name), domain) {
		return uCDNDomain{}, false
	}

	return uCDNDomain{
		Exists: true,
		ID: firstNonEmpty(
			strAny(m["DomainId"]),
			strAny(m["DomainID"]),
			strAny(m["Id"]),
			strAny(m["ID"]),
		),
		CNAME: firstNonEmpty(
			strAny(m["Cname"]),
			strAny(m["CNAME"]),
			strAny(m["CName"]),
			strAny(m["DomainCname"]),
			strAny(m["DomainCNAME"]),
		),
	}, true
}
func createUCloudCDNDomain(publicKey, privateKey, domain, originDomain, projectID, originKey string, scope, originPort int) error {

	params := map[string]string{
		"Domain":         domain,
		"CdnType":        "web",
		"AreaCode":       ucloudAreaCode(scope),
		originKey:        originDomain,
		"OriginHost":     originDomain,
		"OriginPort":     fmt.Sprint(originPort),
		"OriginProtocol": "https",
	}

	for _, action := range []string{"CreateUcdnDomain", "AddUcdnDomain"} {
		p := map[string]string{"Action": action}
		for k, v := range params {
			p[k] = v
		}

		_, err := uCall(publicKey, privateKey, projectID, "", p)
		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "Action") && strings.Contains(err.Error(), "not found") {
			continue
		}

		if strings.Contains(err.Error(), "Service unavailable") {
			continue
		}

		return err
	}

	return fmt.Errorf("UCloud CDN 创建接口不可用，请确认账号已开通 UCDN，或先在控制台手动添加域名")
}
func upsertUCloudCDNDomain(publicKey, privateKey, domain, originDomain, projectID, region string, scope, prot int, isDomain bool) (string, error) {
	domain = normalizeUCloudDomain(domain)
	originDomain = strings.TrimSpace(originDomain)
	if domain == "" {
		return "", fmt.Errorf("UCloud CDN domain is empty")
	}
	if originDomain == "" {
		return "", fmt.Errorf("UCloud CDN origin is empty: %s", domain)
	}

	d, err := getUCloudCDNDomain(publicKey, privateKey, domain, projectID, region)
	if err != nil {
		return "", err
	}

	originPort := prot
	if originPort <= 0 {
		originPort = 80
	}

	originKey := "OriginIpList.0"
	if isDomain {
		originKey = "OriginDomain.0"
	}
	if d.Exists {
		if d.ID == "" {
			return "", fmt.Errorf("UCloud CDN domain id empty: %s", domain)
		}
		_, err := uCall(publicKey, privateKey, projectID, region, map[string]string{
			"Action":         actionUpdateCDN,
			"DomainId":       d.ID,
			originKey:        originDomain,
			"OriginHost":     originDomain,
			"OriginPort":     fmt.Sprint(originPort),
			"OriginProtocol": "https",
		})
		if err != nil {
			return "", err
		}

		if d.CNAME != "" {
			return d.CNAME, nil
		}

		d, _ = getUCloudCDNDomain(publicKey, privateKey, domain, projectID, region)
		return d.CNAME, nil
	}
	err = createUCloudCDNDomain(publicKey, privateKey, domain, originDomain, projectID, originKey, scope, originPort)
	if err != nil {
		return "", err
	}

	time.Sleep(10 * time.Second)

	d, err = getUCloudCDNDomain(publicKey, privateKey, domain, projectID, region)
	if err != nil {
		return "", err
	}
	if d.CNAME == "" {
		return "", fmt.Errorf("UCloud CDN CNAME empty: %s", domain)
	}
	return d.CNAME, nil
}

func UploadUCloudCDNCert(dns DNSRecord, certName, certPEM, keyPEM string) error {
	dns.Domain = normalizeUCloudDomain(dns.Domain)
	if dns.Domain == "" {
		return fmt.Errorf("UCloud CDN cert domain is empty")
	}

	userCert, caCert := splitUCloudCertChain(certPEM)
	uploadName := ucloudCertUploadName(certName, certPEM)
	params := map[string]string{
		"Action":     "AddCertificate",
		"CertName":   uploadName,
		"UserCert":   userCert,
		"PrivateKey": keyPEM,
		"CertType":   "ucdn",
	}
	if caCert != "" {
		params["CaCert"] = caCert
	}

	resp, err := uCall(dns.Key, dns.Secret, dns.ProjectID, dns.Region, params)
	if err != nil {
		return err
	}

	certID := firstNonEmpty(strAny(resp["CertId"]), strAny(resp["CertID"]), strAny(resp["CertificateId"]), strAny(resp["CertificateID"]))
	if certID == "" {
		var lastErr error
		for i := 0; i < 5; i++ {
			certID, lastErr = findUCloudCertID(dns.Key, dns.Secret, uploadName, dns.Domain, dns.ProjectID, dns.Region)
			if certID != "" {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if certID == "" {
			return fmt.Errorf("UCloud cert id not found after upload: cert=%s err=%v", uploadName, lastErr)
		}
	}

	d, err := getUCloudCDNDomain(dns.Key, dns.Secret, dns.Domain, dns.ProjectID, dns.Region)
	if err != nil {
		return err
	}
	if d.ID == "" {
		return fmt.Errorf("UCloud CDN domain not found: %s", dns.Domain)
	}

	return updateUCloudCDNHTTPS(dns, d.ID, certID, uploadName)
}

func splitUCloudCertChain(fullchain string) (userCert, caCert string) {
	fullchain = strings.ReplaceAll(fullchain, "\\n", "\n")
	fullchain = strings.ReplaceAll(fullchain, "\r\n", "\n")
	fullchain = strings.TrimSpace(fullchain)

	var blocks []string
	remaining := fullchain
	for {
		start := strings.Index(remaining, "-----BEGIN CERTIFICATE-----")
		if start < 0 {
			break
		}
		end := strings.Index(remaining[start:], "-----END CERTIFICATE-----")
		if end < 0 {
			break
		}
		end += start + len("-----END CERTIFICATE-----")
		blocks = append(blocks, strings.TrimSpace(remaining[start:end]))
		remaining = remaining[end:]
	}

	if len(blocks) == 0 {
		return fullchain, ""
	}
	userCert = blocks[0]
	if len(blocks) > 1 {
		caCert = strings.Join(blocks[1:], "\n")
	}
	return userCert, caCert
}

func ucloudCertUploadName(certName, certPEM string) string {
	base := sanitizeUCloudName(certName)
	if base == "" {
		base = "allinssl"
	}
	if len(base) > 32 {
		base = base[:32]
	}
	sum := sha256.Sum256([]byte(certPEM))
	return fmt.Sprintf("%s-%s-%d", base, hex.EncodeToString(sum[:])[:16], time.Now().Unix())
}

func sanitizeUCloudName(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		case r == '.' || r == ' ':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-_")
}

func findUCloudCertID(publicKey, privateKey, certName, domain, projectID, region string) (string, error) {
	if domain != "" {
		if id, err := findUCloudCertIDFromList(publicKey, privateKey, certName, domain, projectID, region); err == nil && id != "" {
			return id, nil
		}
	}
	return findUCloudCertIDFromList(publicKey, privateKey, certName, "", projectID, region)
}

func findUCloudCertIDFromList(publicKey, privateKey, certName, domain, projectID, region string) (string, error) {
	params := map[string]string{
		"Action": "GetCertificateBaseInfoList",
	}
	if domain != "" {
		params["Domain"] = domain
	}

	resp, err := uCall(publicKey, privateKey, projectID, region, params)
	if err != nil {
		return "", err
	}
	return extractUCloudCertID(resp, certName)
}

func extractUCloudCertID(resp uResp, certName string) (string, error) {
	var bestID string
	var bestNum float64

	for _, listKey := range []string{"CertList", "Certs", "CertificateList", "Data", "DataSet"} {
		arr, ok := resp[listKey].([]any)
		if !ok {
			continue
		}
		id, num := findUCloudCertIDInList(arr, certName)
		if num > bestNum || (bestID == "" && id != "") {
			bestID = id
			bestNum = num
		}
	}

	for _, v := range resp {
		arr, ok := v.([]any)
		if !ok {
			continue
		}
		id, num := findUCloudCertIDInList(arr, certName)
		if num > bestNum || (bestID == "" && id != "") {
			bestID = id
			bestNum = num
		}
	}

	if bestID == "" {
		return "", fmt.Errorf("UCloud cert not found: %s", certName)
	}
	return bestID, nil
}

func findUCloudCertIDInList(arr []any, certName string) (string, float64) {
	var bestID string
	var bestNum float64
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := firstNonEmpty(strAny(m["CertName"]), strAny(m["certName"]), strAny(m["Name"]), strAny(m["name"]))
		if name != certName {
			continue
		}
		certType := firstNonEmpty(strAny(m["CertType"]), strAny(m["certType"]), strAny(m["Type"]), strAny(m["type"]))
		if certType != "" && !strings.EqualFold(certType, "ucdn") {
			continue
		}
		id := firstNonEmpty(
			strAny(m["CertId"]),
			strAny(m["CertID"]),
			strAny(m["cert_id"]),
			strAny(m["Id"]),
			strAny(m["ID"]),
			strAny(m["id"]),
		)
		if id == "" {
			continue
		}
		num, _ := strconv.ParseFloat(id, 64)
		if num > bestNum || bestID == "" {
			bestID = id
			bestNum = num
		}
	}
	return bestID, bestNum
}

func updateUCloudCDNHTTPS(dns DNSRecord, domainID, certID, certName string) error {
	base := map[string]string{
		"DomainId": domainID,
		"CertName": certName,
		"CertId":   certID,
		"CertType": "ucdn",
	}

	build := func(action string, extra map[string]string) map[string]string {
		params := map[string]string{"Action": action}
		for k, v := range base {
			params[k] = v
		}
		for k, v := range extra {
			params[k] = v
		}
		return params
	}

	var attempts []map[string]string
	switch ucloudAreaCode(dns.Scope) {
	case "cn":
		attempts = append(attempts, build("UpdateUcdnDomainHttpsConfigV2", map[string]string{
			"HttpsStatusCn": "enable",
		}))
	case "abroad":
		attempts = append(attempts, build("UpdateUcdnDomainHttpsConfigV2", map[string]string{
			"HttpsStatusAbroad": "enable",
		}))
	default:
		attempts = append(attempts,
			build("UpdateUcdnDomainHttpsConfigV2", map[string]string{
				"HttpsStatusCn":     "enable",
				"HttpsStatusAbroad": "enable",
			}),
			build("UpdateUcdnDomainHttpsConfigV2", map[string]string{
				"HttpsStatusCn": "enable",
			}),
		)
	}
	attempts = append(attempts, build("UpdateUcdnDomainHttpsConfig", map[string]string{
		"HttpsStatus": "enable",
	}))

	var errs []string
	for _, params := range attempts {
		if _, err := uCall(dns.Key, dns.Secret, dns.ProjectID, dns.Region, params); err == nil {
			return nil
		} else {
			errs = append(errs, err.Error())
		}
	}

	return fmt.Errorf("UCloud CDN HTTPS config failed: %s", strings.Join(errs, "; "))
}

func PostUCloudCDNAndDNS(dns DNSRecord) error {
	dns.Domain = normalizeUCloudDomain(dns.Domain)
	dns.RootDomain = normalizeUCloudDomain(dns.RootDomain)
	if dns.ProjectID == "" {
		projectID, e := getUCloudProjectID(dns.Key, dns.Secret)
		if e == nil {
			dns.ProjectID = projectID
		}
	}
	cname, err := upsertUCloudCDNDomain(dns.Key, dns.Secret, dns.Domain, dns.OriginDomain, dns.ProjectID, dns.Region, dns.Scope, int(dns.Port), dns.IsDomain)
	if err != nil {
		return err
	}

	dns.RecordType = "CNAME"
	dns.RR = ucloudRRForDomain(dns.RootDomain, dns.Domain, dns.SubDomain)
	dns.Content = cname
	dns.TTL = 600
	_, err = DnsUpsertRecord(dns)
	if err != nil {
		fmt.Println("Failed to upsert record:", err)
		return err
	}
	return nil
}

func ucloudAreaCode(scope int) string {
	if scope == 2 {
		return "cn"
	}
	if scope == 1 {
		return "all"
	}
	return "abroad"
}

func strAny(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
