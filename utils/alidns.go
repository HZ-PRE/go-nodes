package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	alidns "github.com/alibabacloud-go/alidns-20150109/v5/client"
	cdn "github.com/alibabacloud-go/cdn-20180510/v5/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/alibabacloud-go/tea/tea"
	"golang.org/x/net/publicsuffix"
)

const (
	aliDNSEndpoint = "alidns.cn-hangzhou.aliyuncs.com"
	aliCDNEndpoint = "cdn.aliyuncs.com"

	aliDefaultTTL        int64 = 600
	aliDefaultOriginPort       = 80
	aliCDNCnameRetries         = 10
	aliCDNCnameInterval        = 5 * time.Second
	aliVerifyTimeout           = 5 * time.Minute

	aliCDNTypeWeb = "web"
)

const aliCDNCacheFileTypes = "html,ts,7z,avi,avif,apk,bin,bmp,bz2,class,css,csv,doc,docx,dmg,ejs,eot,eps,exe,flac,gif,gz,ico,iso,jar,jpg,jpeg,js,mid,midi,mkv,mp3,mp4,ogg,otf,pdf,pict,pls,png,ppt,pptx,ps,rar,svg,svgz,swf,tar,tif,tiff,ttf,webm,webp,woff,woff2,xls,xlsx,zip,zst"

var aliVerifyCodeKeys = []string{
	"verifyCode",
	"verifiCode",
	"VerifyCode",
	"VerifiCode",
	"verify_code",
	"verifi_code",
}

type aliCDNSource struct {
	Content  string `json:"content"`
	Type     string `json:"type"`
	Priority string `json:"priority"`
	Port     int    `json:"port"`
	Weight   string `json:"weight"`
}

type DomainVerifyData struct {
	RootDomain string
	VerifyKey  string
	VerifyCode string
}

type aliCDNFunction struct {
	FunctionName string              `json:"functionName"`
	FunctionArgs []aliCDNFunctionArg `json:"functionArgs"`
}

type aliCDNFunctionArg struct {
	ArgName  string `json:"argName"`
	ArgValue string `json:"argValue"`
}

func normalizeAliDomain(domain string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
}

func normalizeAliRR(domain, rr string) string {
	domain = normalizeAliDomain(domain)
	rr = strings.TrimSuffix(strings.TrimSpace(rr), ".")
	if rr == "" || rr == "@" {
		return "@"
	}
	if strings.EqualFold(rr, domain) {
		return "@"
	}

	suffix := "." + domain
	if domain != "" && strings.HasSuffix(strings.ToLower(rr), suffix) {
		rr = rr[:len(rr)-len(suffix)]
		if rr == "" {
			return "@"
		}
	}
	return rr
}

func normalizeAliRecordType(recordType string) string {
	return strings.ToUpper(strings.TrimSpace(recordType))
}

func aliRecordName(domain, rr string) string {
	domain = normalizeAliDomain(domain)
	rr = normalizeAliRR(domain, rr)
	if rr == "@" {
		return domain
	}
	return rr + "." + domain
}

func aliRRForDomain(rootDomain, domainName, rr string) string {
	rootDomain = normalizeAliDomain(rootDomain)
	if normalizedRR := normalizeAliRR(rootDomain, rr); normalizedRR != "@" {
		return normalizedRR
	}
	return normalizeAliRR(rootDomain, domainName)
}

func aliCDNSourceType(isDomain bool) string {
	if isDomain {
		return "domain"
	}
	return "ipaddr"
}

func aliCDNScope(scope int) string {
	switch scope {
	case 1:
		return "global"
	case 2:
		return "domestic"
	default:
		return "overseas"
	}
}

func aliCDNOriginPort(port int) int {
	if port <= 0 {
		return aliDefaultOriginPort
	}
	return port
}

func aliOpenAPIConfig(id, secret, endpoint string) *openapi.Config {
	return &openapi.Config{
		AccessKeyId:     tea.String(id),
		AccessKeySecret: tea.String(secret),
		Endpoint:        tea.String(endpoint),
	}
}

func nonEmptyAliVerifyValue(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "<nil>"
}

func stringFromAliVerifyContent(content map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := content[key]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if nonEmptyAliVerifyValue(text) {
			return text
		}
	}
	return ""
}

func aliVerifyDataFromContent(domainName string, content map[string]interface{}) (*DomainVerifyData, error) {
	rootDomain := normalizeAliDomain(stringFromAliVerifyContent(content, "RootDomain", "rootDomain"))
	if rootDomain == "" {
		rootDomain = guessRootDomain(domainName)
	}

	verifyKey := stringFromAliVerifyContent(content, "verifyKey", "VerifyKey")
	if verifyKey == "" {
		verifyKey = "aliyunverify"
	}
	verifyKey = normalizeAliRR(rootDomain, verifyKey)

	verifyCode := stringFromAliVerifyContent(content, aliVerifyCodeKeys...)
	if verifyCode == "" {
		return nil, fmt.Errorf("verifyCode not found: %#v", content)
	}

	return &DomainVerifyData{
		RootDomain: rootDomain,
		VerifyKey:  verifyKey,
		VerifyCode: verifyCode,
	}, nil
}

func newAliCDNClient(id, secret string) (*cdn.Client, error) {
	return cdn.NewClient(aliOpenAPIConfig(id, secret, aliCDNEndpoint))
}

func newAliDNSClient(id, secret string) (*alidns.Client, error) {
	return alidns.NewClient(aliOpenAPIConfig(id, secret, aliDNSEndpoint))
}

func findRecord(client *alidns.Client, domain, rr, recordType string) (*alidns.DescribeSubDomainRecordsResponseBodyDomainRecordsRecord, error) {
	domain = normalizeAliDomain(domain)
	rr = normalizeAliRR(domain, rr)
	recordType = normalizeAliRecordType(recordType)

	req := &alidns.DescribeSubDomainRecordsRequest{
		DomainName: tea.String(domain),
		SubDomain:  tea.String(aliRecordName(domain, rr)),
		Type:       tea.String(recordType),
		PageNumber: tea.Int64(1),
		PageSize:   tea.Int64(100),
	}

	resp, err := client.DescribeSubDomainRecords(req)
	if err != nil {
		return nil, err
	}
	if resp.Body == nil || resp.Body.DomainRecords == nil {
		return nil, nil
	}

	for _, r := range resp.Body.DomainRecords.Record {
		if r != nil && strings.EqualFold(tea.StringValue(r.RR), rr) && strings.EqualFold(tea.StringValue(r.Type), recordType) {
			return r, nil
		}
	}
	return nil, nil
}

func updateRecord(client *alidns.Client, recordID, rr, recordType, newValue string, ttl int64) error {
	_, err := client.UpdateDomainRecord(&alidns.UpdateDomainRecordRequest{
		RecordId: tea.String(recordID),
		RR:       tea.String(rr),
		Type:     tea.String(recordType),
		Value:    tea.String(newValue),
		TTL:      tea.Int64(ttl),
	})
	return err
}

func addRecord(client *alidns.Client, domain, rr, recordType, value string, ttl int64) error {
	_, err := client.AddDomainRecord(&alidns.AddDomainRecordRequest{
		DomainName: tea.String(domain),
		RR:         tea.String(rr),
		Type:       tea.String(recordType),
		Value:      tea.String(value),
		TTL:        tea.Int64(ttl),
	})
	return err
}

func AliUpsertRecord(id, secret, domain, rr, recordType, value string, ttl int64) (bool, error) {
	domain = normalizeAliDomain(domain)
	rr = normalizeAliRR(domain, rr)
	recordType = normalizeAliRecordType(recordType)
	value = strings.TrimSpace(value)
	if ttl <= 0 {
		ttl = aliDefaultTTL
	}
	if domain == "" {
		return true, fmt.Errorf("阿里云 DNS 根域名不能为空")
	}
	if recordType == "" {
		return true, fmt.Errorf("阿里云 DNS 记录类型不能为空")
	}
	if value == "" {
		return true, fmt.Errorf("阿里云 DNS 记录值不能为空: %s %s", aliRecordName(domain, rr), recordType)
	}

	client, err := newAliDNSClient(id, secret)
	if err != nil {
		return true, err
	}
	record, err := findRecord(client, domain, rr, recordType)
	if err != nil {
		return true, err
	}

	recordName := aliRecordName(domain, rr)
	if record == nil {
		fmt.Printf("阿里云 DNS 记录不存在，创建: %s %s -> %s\n", recordName, recordType, value)
		return true, addRecord(client, domain, rr, recordType, value, ttl)
	}

	oldValue := tea.StringValue(record.Value)
	oldTTL := tea.Int64Value(record.TTL)
	if oldValue == value && oldTTL == ttl {
		return false, nil
	}

	recordID := tea.StringValue(record.RecordId)
	if recordID == "" {
		return true, fmt.Errorf("阿里云 DNS 记录 ID 为空: %s %s", recordName, recordType)
	}
	fmt.Printf("阿里云 DNS 记录存在，更新: %s %s %s -> %s\n", recordName, recordType, oldValue, value)
	return true, updateRecord(client, recordID, rr, recordType, value, ttl)
}

func findAliCDNDomain(client *cdn.Client, domain string) (*cdn.DescribeUserDomainsResponseBodyDomainsPageData, error) {
	domain = normalizeAliDomain(domain)
	if domain == "" {
		return nil, fmt.Errorf("阿里 CDN 域名不能为空")
	}

	resp, err := client.DescribeUserDomains(&cdn.DescribeUserDomainsRequest{
		DomainName:       tea.String(domain),
		DomainSearchType: tea.String("full_match"),
		PageNumber:       tea.Int32(1),
		PageSize:         tea.Int32(50),
	})
	if err != nil {
		return nil, err
	}
	if resp.Body == nil || resp.Body.Domains == nil {
		return nil, nil
	}

	for _, item := range resp.Body.Domains.PageData {
		if item != nil && strings.EqualFold(tea.StringValue(item.DomainName), domain) {
			return item, nil
		}
	}
	return nil, nil
}

func domainExists(client *cdn.Client, domain string) (bool, error) {
	domainInfo, err := findAliCDNDomain(client, domain)
	if err != nil {
		return false, err
	}
	return domainInfo != nil, nil
}

func parseAliVerifyContent(domainName, rawContent string) (*DomainVerifyData, error) {
	rawContent = strings.TrimSpace(rawContent)
	if rawContent == "" {
		return nil, fmt.Errorf("阿里 CDN 域名验证内容为空: %s", domainName)
	}

	var content map[string]interface{}
	if err := json.Unmarshal([]byte(rawContent), &content); err != nil {
		return nil, fmt.Errorf("解析阿里 CDN 域名验证内容失败: %w", err)
	}

	verifyData, err := aliVerifyDataFromContent(domainName, content)
	if err != nil {
		return nil, fmt.Errorf("阿里 CDN %w", err)
	}
	return verifyData, nil
}

func describeDomainVerifyData(id, secret, domainName string) (*DomainVerifyData, error) {
	domainName = normalizeAliDomain(domainName)

	client, err := openapi.NewClient(aliOpenAPIConfig(id, secret, aliCDNEndpoint))
	if err != nil {
		return nil, err
	}

	params := &openapiutil.Params{
		Action:      tea.String("DescribeDomainVerifyData"),
		Version:     tea.String("2018-05-10"),
		Protocol:    tea.String("HTTPS"),
		Pathname:    tea.String("/"),
		Method:      tea.String("POST"),
		AuthType:    tea.String("AK"),
		Style:       tea.String("RPC"),
		ReqBodyType: tea.String("json"),
		BodyType:    tea.String("json"),
	}

	request := &openapiutil.OpenApiRequest{
		Query: map[string]*string{
			"DomainName": tea.String(domainName),
		},
	}

	resp, err := client.CallApi(params, request, &dara.RuntimeOptions{})
	if err != nil {
		return nil, err
	}

	body, ok := resp["body"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("body not found: %#v", resp)
	}
	content, ok := body["Content"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Content not object: %#v", body["Content"])
	}

	return aliVerifyDataFromContent(domainName, content)
}

func guessRootDomain(domainName string) string {
	domainName = normalizeAliDomain(domainName)
	if root, err := publicsuffix.EffectiveTLDPlusOne(domainName); err == nil {
		return root
	}
	parts := strings.Split(domainName, ".")
	if len(parts) < 2 {
		return domainName
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func retryGetAliCDNCname(client *cdn.Client, domainName string) (string, error) {
	var lastErr error
	for i := 0; i < aliCDNCnameRetries; i++ {
		cname, err := getCDNCname(client, domainName)
		if err == nil && cname != "" {
			return cname, nil
		}
		lastErr = err
		time.Sleep(aliCDNCnameInterval)
	}
	return "", lastErr
}

func getCDNCname(client *cdn.Client, domainName string) (string, error) {
	domainName = normalizeAliDomain(domainName)
	resp, err := client.DescribeCdnDomainDetail(&cdn.DescribeCdnDomainDetailRequest{
		DomainName: tea.String(domainName),
	})
	if err != nil {
		return "", err
	}
	if resp.Body == nil || resp.Body.GetDomainDetailModel == nil {
		return "", fmt.Errorf("CDN domain detail not found")
	}

	cname := strings.TrimSpace(tea.StringValue(resp.Body.GetDomainDetailModel.Cname))
	if cname == "" {
		return "", fmt.Errorf("阿里 CDN CNAME 为空: %s", domainName)
	}
	return cname, nil
}

func DescribeVerifyContent(id, secret, domainName string) (string, error) {
	client, err := newAliCDNClient(id, secret)
	if err != nil {
		return "", err
	}

	resp, err := client.DescribeVerifyContent(&cdn.DescribeVerifyContentRequest{
		DomainName: tea.String(domainName),
	})
	if err != nil {
		return "", err
	}
	if resp.Body == nil || tea.StringValue(resp.Body.Content) == "" {
		return "", fmt.Errorf("verify content not found")
	}
	return tea.StringValue(resp.Body.Content), nil
}

func verifyDomainOwner(client *cdn.Client, domainName string) error {
	domainName = normalizeAliDomain(domainName)
	_, err := client.VerifyDomainOwner(&cdn.VerifyDomainOwnerRequest{
		DomainName: tea.String(domainName),
		VerifyType: tea.String("dnsCheck"),
	})
	return err
}

func aliFunctionArg(name, value string) aliCDNFunctionArg {
	return aliCDNFunctionArg{ArgName: name, ArgValue: value}
}

func marshalAliCDNFunctions(origin, forwardScheme string, port int) (string, error) {
	functions := []aliCDNFunction{
		{
			FunctionName: "filetype_based_ttl_set",
			FunctionArgs: []aliCDNFunctionArg{
				aliFunctionArg("file_type", aliCDNCacheFileTypes),
				aliFunctionArg("ttl", "86400"),
				aliFunctionArg("weight", "99"),
				aliFunctionArg("swift_origin_cache_high", "off"),
				aliFunctionArg("swift_no_cache_low", "off"),
				aliFunctionArg("swift_follow_cachetime", "off"),
				aliFunctionArg("force_revalidate", "off"),
			},
		},
		{
			FunctionName: "gzip",
			FunctionArgs: []aliCDNFunctionArg{
				aliFunctionArg("enable", "on"),
			},
		},
		{
			FunctionName: "brotli",
			FunctionArgs: []aliCDNFunctionArg{
				aliFunctionArg("enable", "on"),
			},
		},
		{
			FunctionName: "forward_scheme",
			FunctionArgs: []aliCDNFunctionArg{
				aliFunctionArg("enable", "on"),
				aliFunctionArg("scheme_origin", forwardScheme),
				aliFunctionArg("scheme_origin_port", fmt.Sprint(port)),
			},
		},
		{
			FunctionName: "set_req_host_header",
			FunctionArgs: []aliCDNFunctionArg{
				aliFunctionArg("domain_name", origin),
			},
		},
		{
			FunctionName: "https_origin_sni",
			FunctionArgs: []aliCDNFunctionArg{
				aliFunctionArg("enabled", "on"),
				aliFunctionArg("https_origin_sni", origin),
			},
		},
		{
			FunctionName: "https_tls_version",
			FunctionArgs: []aliCDNFunctionArg{
				aliFunctionArg("tls13", "on"),
				aliFunctionArg("tls12", "on"),
				aliFunctionArg("tls11", "on"),
				aliFunctionArg("tls10", "on"),
			},
		},
		{
			FunctionName: "range",
			FunctionArgs: []aliCDNFunctionArg{
				aliFunctionArg("enable", "on"),
			},
		},
	}

	data, err := json.Marshal(functions)
	if err != nil {
		return "", fmt.Errorf("生成阿里 CDN 配置失败: %w", err)
	}
	return string(data), nil
}

func updateAliCDNRules(client *cdn.Client, domainName, origin string, port int, isDomain bool) error {
	// describeAliCDNAllConfig(client, domainName)
	forwardScheme := "http"
	if isDomain {
		forwardScheme = "https"
	}

	functions, err := marshalAliCDNFunctions(origin, forwardScheme, port)
	if err != nil {
		return err
	}

	_, err = client.BatchSetCdnDomainConfig(&cdn.BatchSetCdnDomainConfigRequest{
		DomainNames: tea.String(domainName),
		Functions:   tea.String(functions),
	})
	return err
}

// 获取可用参数
// func describeAliCDNAllConfig(client *cdn.Client, domainName string) error {
// 	req := &cdn.DescribeCdnDomainConfigsRequest{
// 		DomainName: tea.String(domainName),
// 	}
//
// 	resp, err := client.DescribeCdnDomainConfigs(req)
// 	if err != nil {
// 		return err
// 	}
//
// 	b, _ := json.MarshalIndent(resp, "", "  ")
// 	fmt.Println(string(b))
// 	return nil
// }

func updateCDNOrigin(client *cdn.Client, domainName, origin string, scope, port int, isDomain bool) (string, error) {
	domainName = normalizeAliDomain(domainName)
	origin = strings.TrimSpace(origin)
	if domainName == "" {
		return "", fmt.Errorf("阿里 CDN 域名不能为空")
	}
	if origin == "" {
		return "", fmt.Errorf("阿里 CDN 源站不能为空: %s", domainName)
	}

	port = aliCDNOriginPort(port)
	sourceJSON, err := json.Marshal([]aliCDNSource{
		{
			Content:  origin,
			Type:     aliCDNSourceType(isDomain),
			Priority: "20",
			Port:     port,
			Weight:   "15",
		},
	})
	if err != nil {
		return "", err
	}

	domainInfo, err := findAliCDNDomain(client, domainName)
	if err != nil {
		return "", err
	}

	if domainInfo != nil {
		fmt.Println("CDN 域名已存在，更新源站")
		_, err = client.ModifyCdnDomain(&cdn.ModifyCdnDomainRequest{
			DomainName: tea.String(domainName),
			Sources:    tea.String(string(sourceJSON)),
		})
		if err != nil {
			if strings.Contains(err.Error(), "ServiceBusy") {
				log.Printf("阿里服务器繁忙，无法修改CDN域名源站: %v", err)
				return "", fmt.Errorf("阿里服务器繁忙，请等1分钟再试")
			}
			return "", err
		}
		if err := updateAliCDNRules(client, domainName, origin, port, isDomain); err != nil {
			return "", err
		}
		return retryGetAliCDNCname(client, domainName)
	}

	fmt.Println("CDN 域名不存在，自动创建")
	_, err = client.AddCdnDomain(&cdn.AddCdnDomainRequest{
		DomainName: tea.String(domainName),
		Sources:    tea.String(string(sourceJSON)),
		CdnType:    tea.String(aliCDNTypeWeb),
		Scope:      tea.String(aliCDNScope(scope)), // domestic 仅中国内地，overseas 全球不含中国内地，global 全球
	})
	if err != nil {
		if strings.Contains(err.Error(), "DomainNotRegistration") {
			root := guessRootDomain(domainName)
			log.Printf("根域名未注册或阿里云未识别: %s，当前加速域名: %s: %v", root, domainName, err)
			return "", fmt.Errorf("根域名未注册或阿里云未识别: %s，当前加速域名: %s", root, domainName)
		}
		if strings.Contains(err.Error(), "DomainInBlacklist") {
			log.Printf("域名被阿里云 CDN 黑名单限制: %s", err)
			return "", fmt.Errorf("域名被阿里云 CDN 黑名单限制")
		}
		return "", err
	}

	time.Sleep(aliCDNCnameInterval)
	if err := updateAliCDNRules(client, domainName, origin, port, isDomain); err != nil {
		return "", err
	}
	return retryGetAliCDNCname(client, domainName)
}

func UploadAliCDNCert(id, secret, domainName, certName, certPem, keyPem string) error {
	domainName = normalizeAliDomain(domainName)
	if domainName == "" {
		return fmt.Errorf("阿里 CDN 证书域名不能为空")
	}

	client, err := newAliCDNClient(id, secret)
	if err != nil {
		return err
	}

	_, err = client.SetCdnDomainSSLCertificate(&cdn.SetCdnDomainSSLCertificateRequest{
		DomainName:  tea.String(domainName),
		SSLProtocol: tea.String("on"),
		CertType:    tea.String("upload"),
		CertName:    tea.String(certName),
		SSLPub:      tea.String(certPem),
		SSLPri:      tea.String(keyPem),
	})
	return err
}

func PostAliCdnDomain(dns DNSRecord) error {
	dns.Domain = normalizeAliDomain(dns.Domain)
	if dns.RootDomain == "" {
		dns.RootDomain = guessRootDomain(dns.Domain)
	} else {
		dns.RootDomain = normalizeAliDomain(dns.RootDomain)
	}

	client, err := newAliCDNClient(dns.Key, dns.Secret)
	if err != nil {
		return err
	}

	domainInfo, err := findAliCDNDomain(client, dns.Domain)
	if err != nil {
		return err
	}

	if domainInfo == nil {
		verifyData, err := describeDomainVerifyData(dns.Key, dns.Secret, dns.Domain)
		if err != nil {
			fmt.Println("Failed to get verify data:", err)
			return err
		}

		fmt.Printf("Verify TXT: %s.%s -> %s\n", verifyData.VerifyKey, verifyData.RootDomain, verifyData.VerifyCode)
		dns.RecordType = "TXT"
		dns.RR = verifyData.VerifyKey
		dns.Content = verifyData.VerifyCode
		dns.TTL = aliDefaultTTL
		dns.RootDomain = verifyData.RootDomain
		flag, err := DnsUpsertRecord(dns)
		if err != nil {
			fmt.Println("Failed to upsert record:", err)
			return err
		}
		if flag {
			txtName := fmt.Sprintf("%s.%s", verifyData.VerifyKey, verifyData.RootDomain)
			if err := WaitTXT(txtName, verifyData.VerifyCode, aliVerifyTimeout); err != nil {
				fmt.Println("Failed to wait for TXT record:", err)
				return err
			}
		}

		if err := verifyDomainOwner(client, dns.Domain); err != nil {
			fmt.Println("无法验证CDN域名所有者:", err)
			return err
		}
	}

	cname, err := updateCDNOrigin(client, dns.Domain, dns.OriginDomain, dns.Scope, int(dns.Port), dns.IsDomain)
	if err != nil {
		fmt.Println("Failed to update CDN origin:", err)
		return err
	}
	if cname == "" {
		return fmt.Errorf("阿里 CDN CNAME 为空: %s", dns.Domain)
	}

	dns.RR = aliRRForDomain(dns.RootDomain, dns.Domain, dns.SubDomain)
	dns.RecordType = "CNAME"
	dns.Content = cname
	dns.TTL = aliDefaultTTL
	_, err = DnsUpsertRecord(dns)
	if err != nil {
		fmt.Println("Failed to upsert record:", err)
		return err
	}
	return nil
}
