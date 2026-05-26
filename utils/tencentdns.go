package utils

import (
	"fmt"
	"strings"
	"time"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"

	cdn "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cdn/v20180606"
	cdnm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cdn/v20180606"

	dnsm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"

	ssl "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ssl/v20191205"
	sslm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ssl/v20191205"
)

type TencentCDNConfig struct {
	SecretID  string
	SecretKey string

	RootDomain string // example.com
	SubDomain  string // cdn
	Domain     string // cdn.example.com

	OriginDomain string
	OriginPort   int
	Scope        int // 0 海外, 1 全球, 2 国内

	CertName string
	CertPEM  string
	KeyPEM   string
}

func newTencentCDNClient(id, key string) (*cdn.Client, error) {
	cred := common.NewCredential(id, key)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "cdn.tencentcloudapi.com"
	return cdn.NewClient(cred, "", cpf)
}

func newTencentDNSPodClient(id, key string) (*dnspod.Client, error) {
	cred := common.NewCredential(id, key)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "dnspod.tencentcloudapi.com"
	return dnspod.NewClient(cred, "", cpf)
}

func newTencentSSLClient(id, key string) (*ssl.Client, error) {
	cred := common.NewCredential(id, key)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "ssl.tencentcloudapi.com"
	return ssl.NewClient(cred, "", cpf)
}

func getTencentCDNVerifyTXT(client *cdn.Client, domain, rootDomain string) (rr, value string, err error) {
	req := cdnm.NewCreateVerifyRecordRequest()
	req.Domain = &domain

	resp, err := client.CreateVerifyRecord(req)
	if err != nil {
		return "", "", err
	}

	if resp.Response == nil || resp.Response.SubDomain == nil || resp.Response.Record == nil {
		return "", "", fmt.Errorf("腾讯 CDN 验证信息为空")
	}

	rr = strings.TrimSuffix(*resp.Response.SubDomain, "."+rootDomain)
	rr = strings.TrimSuffix(rr, ".")
	value = *resp.Response.Record

	return rr, value, nil
}

func UpsertTencentDNSRecord(secretID, secretKey, rootDomain, rr, recordType, value string, ttl uint64) (bool, error) {
	client, err := newTencentDNSPodClient(secretID, secretKey)
	if err != nil {
		return true, err
	}

	if ttl <= 0 {
		ttl = 600
	}

	recordLine := "默认"

	listReq := dnsm.NewDescribeRecordListRequest()
	listReq.Domain = &rootDomain
	listReq.Subdomain = &rr
	listReq.RecordType = &recordType

	listResp, err := client.DescribeRecordList(listReq)
	if err != nil {
		return true, err
	}

	if listResp.Response != nil && listResp.Response.RecordList != nil {
		for _, r := range listResp.Response.RecordList {
			if r == nil || r.RecordId == nil {
				continue
			}

			oldValue := ""
			if r.Value != nil {
				oldValue = *r.Value
			}

			if oldValue == value {
				return false, nil
			}

			modReq := dnsm.NewModifyRecordRequest()
			modReq.Domain = &rootDomain
			modReq.RecordId = r.RecordId
			modReq.SubDomain = &rr
			modReq.RecordType = &recordType
			modReq.RecordLine = &recordLine
			modReq.Value = &value
			modReq.TTL = &ttl

			_, err = client.ModifyRecord(modReq)
			return true, err
		}
	}

	addReq := dnsm.NewCreateRecordRequest()
	addReq.Domain = &rootDomain
	addReq.SubDomain = &rr
	addReq.RecordType = &recordType
	addReq.RecordLine = &recordLine
	addReq.Value = &value
	addReq.TTL = &ttl

	_, err = client.CreateRecord(addReq)
	return true, err
}
func verifyTencentCDNDomain(client *cdn.Client, domain string) error {
	req := cdnm.NewVerifyDomainRecordRequest()
	req.Domain = &domain

	_, err := client.VerifyDomainRecord(req)
	return err
}
func updateTencentCDNCacheRules(client *cdn.Client, domain string) error {
	req := cdnm.NewUpdateDomainConfigRequest()
	req.Domain = &domain
	extensions := []*string{
		tea.String("html"),
		tea.String("ts"),
		tea.String("7z"),
		tea.String("avi"),
		tea.String("avif"),
		tea.String("apk"),
		tea.String("bin"),
		tea.String("bmp"),
		tea.String("bz2"),
		tea.String("class"),
		tea.String("css"),
		tea.String("csv"),
		tea.String("doc"),
		tea.String("docx"),
		tea.String("dmg"),
		tea.String("ejs"),
		tea.String("eot"),
		tea.String("eps"),
		tea.String("exe"),
		tea.String("flac"),
		tea.String("gif"),
		tea.String("gz"),
		tea.String("ico"),
		tea.String("iso"),
		tea.String("jar"),
		tea.String("jpg"),
		tea.String("jpeg"),
		tea.String("js"),
		tea.String("mid"),
		tea.String("midi"),
		tea.String("mkv"),
		tea.String("mp3"),
		tea.String("mp4"),
		tea.String("ogg"),
		tea.String("otf"),
		tea.String("pdf"),
		tea.String("pict"),
		tea.String("pls"),
		tea.String("png"),
		tea.String("ppt"),
		tea.String("pptx"),
		tea.String("ps"),
		tea.String("rar"),
		tea.String("svg"),
		tea.String("svgz"),
		tea.String("swf"),
		tea.String("tar"),
		tea.String("tif"),
		tea.String("tiff"),
		tea.String("ttf"),
		tea.String("webm"),
		tea.String("webp"),
		tea.String("woff"),
		tea.String("woff2"),
		tea.String("xls"),
		tea.String("xlsx"),
		tea.String("zip"),
		tea.String("zst"),
	}
	zipTypes := []*string{
		tea.String("html"),
		tea.String("css"),
		tea.String("js"),
		tea.String("mjs"),
		tea.String("json"),
		tea.String("xml"),
		tea.String("txt"),
		tea.String("csv"),
		tea.String("svg"),
		tea.String("svgz"),
		tea.String("ejs"),
		tea.String("ttf"),
		tea.String("otf"),
		tea.String("woff"),
	}
	req.SpecificConfig = &cdnm.SpecificConfig{
		Overseas: &cdnm.OverseaConfig{
			IpFreqLimit: &cdnm.IpFreqLimit{
				Switch: tea.String("on"),
				Qps:    tea.Int64(100),
			},
		},
	}
	req.IpFreqLimit = &cdnm.IpFreqLimit{
		Switch: tea.String("on"),
		Qps:    tea.Int64(100),
	}
	req.Compression = &cdnm.Compression{
		Switch: tea.String("on"),
		CompressionRules: []*cdnm.CompressionRule{
			{
				Compress: tea.Bool(true),
				Algorithms: []*string{
					tea.String("gzip"),
					tea.String("brotli"),
				},
				FileExtensions: zipTypes,
			},
		},
	}
	cacheRules := []*cdnm.RuleCache{
		{
			RuleType:  tea.String("file"),
			RulePaths: extensions,
			CacheConfig: &cdnm.RuleCacheConfig{
				Cache: &cdnm.CacheConfigCache{
					Switch:    tea.String("on"),
					CacheTime: tea.Int64(86400),
				},
			},
		},
	}

	req.Cache = &cdnm.Cache{
		RuleCache: cacheRules,
	}

	_, err := client.UpdateDomainConfig(req)
	return err
}
func tencentCDNExists(client *cdn.Client, domain string) (bool, error) {
	req := cdnm.NewDescribeDomainsConfigRequest()
	req.Filters = []*cdnm.DomainFilter{
		{
			Name:  tea.String("domain"),
			Value: []*string{&domain},
		},
	}

	resp, err := client.DescribeDomainsConfig(req)
	if err != nil {
		return false, err
	}

	return resp.Response != nil &&
		resp.Response.Domains != nil &&
		len(resp.Response.Domains) > 0, nil
}
func tencentArea(scope int) string {
	switch scope {
	case 1:
		return "global"
	case 2:
		return "mainland"
	default:
		return "overseas"
	}
}
func upsertTencentCDNDomain(client *cdn.Client, domain, originDomain string, isDomain bool, scope int) (string, error) {
	exists, err := tencentCDNExists(client, domain)
	if err != nil {
		return "", err
	}

	originType := "ip"
	originProtocol := "http"
	if isDomain {
		originType = "domain"
		originProtocol = "https"
	}
	origin := cdnm.Origin{
		Origins:            []*string{&originDomain},
		ServerName:         &originDomain,
		OriginType:         &originType,
		OriginPullProtocol: &originProtocol,
		Sni: &cdnm.OriginSni{
			Switch:     tea.String("on"),
			ServerName: &originDomain,
		},
	}
	if exists {
		req := cdnm.NewUpdateDomainConfigRequest()
		req.Domain = &domain
		req.Origin = &origin
		_, err = client.UpdateDomainConfig(req)
		if err != nil {
			return "", err
		}
	} else {
		area := tencentArea(scope)
		req := cdnm.NewAddCdnDomainRequest()
		req.Domain = &domain
		req.ServiceType = tea.String("web")
		req.Area = &area
		req.Origin = &origin
		_, err = client.AddCdnDomain(req)
		if err != nil {
			return "", err
		}

		time.Sleep(10 * time.Second)
	}
	if err := updateTencentCDNCacheRules(client, domain); err != nil {
		return "", err
	}
	return getTencentCDNCNAME(client, domain)
}

func getTencentCDNCNAME(client *cdn.Client, domain string) (string, error) {
	req := cdnm.NewDescribeDomainsConfigRequest()
	req.Filters = []*cdnm.DomainFilter{
		{
			Name:  tea.String("domain"),
			Value: []*string{&domain},
		},
	}

	resp, err := client.DescribeDomainsConfig(req)
	if err != nil {
		return "", err
	}

	if resp.Response == nil || len(resp.Response.Domains) == 0 {
		return "", fmt.Errorf("腾讯 CDN 域名不存在: %s", domain)
	}

	d := resp.Response.Domains[0]
	if d.Cname == nil || *d.Cname == "" {
		return "", fmt.Errorf("腾讯 CDN CNAME 为空: %s", domain)
	}

	return *d.Cname, nil
}

func UploadTencentCDNCert(dns DNSRecord, certName, certPem, keyPem string) error {
	sslClient, err := newTencentSSLClient(dns.Key, dns.Secret)
	if err != nil {
		return err
	}
	uploadReq := sslm.NewUploadCertificateRequest()
	uploadReq.CertificatePublicKey = &certPem
	uploadReq.CertificatePrivateKey = &keyPem
	uploadReq.Alias = &certName

	uploadResp, err := sslClient.UploadCertificate(uploadReq)
	if err != nil {
		return err
	}

	if uploadResp.Response == nil || uploadResp.Response.CertificateId == nil {
		return fmt.Errorf("腾讯云证书上传成功但 CertificateId 为空")
	}

	certID := *uploadResp.Response.CertificateId

	cdnClient, err := newTencentCDNClient(dns.Key, dns.Secret)
	if err != nil {
		return err
	}

	httpsSwitch := "on"
	req := cdnm.NewUpdateDomainConfigRequest()
	req.Domain = &dns.Domain
	req.Https = &cdnm.Https{
		Switch: &httpsSwitch,
		CertInfo: &cdnm.ServerCert{
			CertId: &certID,
		},
	}

	_, err = cdnClient.UpdateDomainConfig(req)
	return err
}

func PostTencentCDNAndDNS(dns DNSRecord) error {
	client, err := newTencentCDNClient(dns.Key, dns.Secret)
	if err != nil {
		return err
	}
	rr, txt, err := getTencentCDNVerifyTXT(client, dns.Domain, dns.RootDomain)
	if err != nil {
		return err
	}
	dns.RecordType = "TXT"
	dns.RR = rr
	dns.Content = txt
	dns.TTL = 600
	changed, err := DnsUpsertRecord(dns)
	if err != nil {
		return err
	}

	if changed {
		if err := WaitTXT(rr+"."+dns.RootDomain, txt, 5*time.Minute); err != nil {
			return err
		}
	}

	if err := verifyTencentCDNDomain(client, dns.Domain); err != nil {
		return err
	}

	cname, err := upsertTencentCDNDomain(client, dns.Domain, dns.OriginDomain, dns.IsDomain, dns.Scope)
	if err != nil {
		return err
	}

	cnameRR := dns.SubDomain
	if cnameRR == "" {
		cnameRR = "@"
	}
	dns.RecordType = "CNAME"
	dns.Content = cname
	dns.TTL = 600
	dns.RR = cnameRR
	if _, err := DnsUpsertRecord(dns); err != nil {
		return err
	}
	return nil
}
