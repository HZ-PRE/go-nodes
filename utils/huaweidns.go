package utils

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/global"

	cdn "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/cdn/v2"
	cdnmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/cdn/v2/model"
	cdnregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/cdn/v2/region"

	dns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	dnsmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	dnsregion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"
)

var (
	huaweiCDNRegions = []string{
		"cn-north-1",
		"ap-southeast-1",
		"eu-west-101",
	}

	huaweiDNSRegions = []string{
		"cn-north-1",
		"cn-north-4",
		"ap-southeast-1",
	}

	errHuaweiCDNDomainNotFound = errors.New("华为云 CDN 域名不存在")
)

func newHuaweiDNSClient(ak, sk string) (*dns.DnsClient, error) {
	auth, err := basic.NewCredentialsBuilder().
		WithAk(ak).
		WithSk(sk).
		SafeBuild()
	if err != nil {
		return nil, err
	}

	var lastErr error
	for _, regionName := range huaweiDNSRegions {
		r, err := dnsregion.SafeValueOf(regionName)
		if err != nil {
			lastErr = err
			continue
		}
		hcClient, err := dns.DnsClientBuilder().
			WithRegion(r).
			WithCredential(auth).
			SafeBuild()
		if err != nil {
			lastErr = err
			continue
		}
		client := dns.NewDnsClient(hcClient)

		// 测试接口
		_, err = client.ListPublicZones(&dnsmodel.ListPublicZonesRequest{
			Limit: tea.Int32(1),
		})
		if err == nil {
			return client, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("未找到可用的华为DNS区域: %w", lastErr)
	}
	return nil, fmt.Errorf("未找到可用的华为DNS区域")
}

func newHuaweiCDNClient(ak, sk string) (*cdn.CdnClient, error) {
	auth, err := global.NewCredentialsBuilder().
		WithAk(ak).
		WithSk(sk).
		SafeBuild()
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, regionName := range huaweiCDNRegions {
		r, err := cdnregion.SafeValueOf(regionName)
		if err != nil {
			lastErr = err
			continue
		}
		hcClient, err := cdn.CdnClientBuilder().
			WithRegion(r).
			WithCredential(auth).
			SafeBuild()
		if err != nil {
			fmt.Printf("华为 CDN 区域 %s 客户端创建失败: %v\n", regionName, err)
			lastErr = err
			continue
		}
		client := cdn.NewCdnClient(hcClient)

		// 测试接口
		_, err = client.ListDomains(
			&cdnmodel.ListDomainsRequest{},
		)
		if err == nil {
			return client, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("未找到可用的华为CDN区域: %w", lastErr)
	}
	return nil, fmt.Errorf("未找到可用的华为CDN区域")
}

func fqdn(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".")
	if name == "" {
		return "."
	}
	return name + "."
}

func huaweiRecordFullName(rootDomain, rr string) string {
	rootDomain = strings.TrimSuffix(strings.TrimSpace(rootDomain), ".")
	rr = strings.TrimSuffix(strings.TrimSpace(rr), ".")
	if rr == "" || rr == "@" || strings.EqualFold(rr, rootDomain) {
		return rootDomain
	}
	if strings.HasSuffix(strings.ToLower(rr), "."+strings.ToLower(rootDomain)) {
		return rr
	}
	return rr + "." + rootDomain
}

func getHuaweiDNSZoneID(client *dns.DnsClient, rootDomain string) (string, error) {
	rootDomain = strings.TrimSuffix(strings.TrimSpace(rootDomain), ".")
	if rootDomain == "" {
		return "", fmt.Errorf("华为云 DNS 根域名为空")
	}
	zoneName := fqdn(rootDomain)
	req := &dnsmodel.ListPublicZonesRequest{
		Type:       tea.String("public"),
		Name:       tea.String(zoneName),
		SearchMode: tea.String("equal"),
		Limit:      tea.Int32(100),
	}

	resp, err := client.ListPublicZones(req)
	if err != nil {
		return "", err
	}

	if resp.Zones == nil || len(*resp.Zones) == 0 {
		return "", fmt.Errorf("华为云 DNS 公网域名不存在: %s", rootDomain)
	}

	for _, zone := range *resp.Zones {
		if zone.Id != nil && zone.Name != nil && strings.EqualFold(*zone.Name, zoneName) {
			return *zone.Id, nil
		}
	}

	return "", fmt.Errorf("华为云 DNS 公网域名不存在: %s", rootDomain)
}

func findHuaweiRecordSet(client *dns.DnsClient, zoneID, fullName, recordType string) (*dnsmodel.ListRecordSets, error) {
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	targetName := fqdn(fullName)
	req := &dnsmodel.ListRecordSetsByZoneRequest{
		ZoneId:     zoneID,
		SearchMode: tea.String("equal"),
		Limit:      tea.Int32(100),
		Name:       tea.String(targetName),
		Type:       tea.String(recordType),
	}

	resp, err := client.ListRecordSetsByZone(req)
	if err != nil {
		return nil, err
	}

	if resp.Recordsets == nil {
		return nil, nil
	}

	for _, r := range *resp.Recordsets {
		if r.Name != nil && strings.EqualFold(*r.Name, targetName) &&
			r.Type != nil && *r.Type == recordType {
			record := r
			return &record, nil
		}
	}

	return nil, nil
}

func UpsertHuaweiDNSRecord(ak, sk, rootDomain, rr, recordType, value string, ttl int32) (bool, error) {
	if ttl <= 0 {
		ttl = 300
	}
	recordType = strings.ToUpper(strings.TrimSpace(recordType))
	client, err := newHuaweiDNSClient(ak, sk)
	if err != nil {
		return true, err
	}

	zoneID, err := getHuaweiDNSZoneID(client, rootDomain)
	if err != nil {
		return true, err
	}
	fullName := huaweiRecordFullName(rootDomain, rr)

	record, err := findHuaweiRecordSet(client, zoneID, fullName, recordType)
	if err != nil {
		return true, err
	}

	records := []string{value}

	if record == nil {
		fmt.Printf("华为云 DNS 记录不存在，创建: %s %s -> %s\n", fullName, recordType, value)

		req := &dnsmodel.CreateRecordSetRequest{
			ZoneId: zoneID,
			Body: &dnsmodel.CreateRecordSetRequestBody{
				Name:    fqdn(fullName),
				Type:    recordType,
				Records: records,
				Ttl:     &ttl,
			},
		}

		_, err = client.CreateRecordSet(req)
		return true, err
	}
	oldValue := ""
	if record.Records != nil && len(*record.Records) > 0 {
		oldValue = (*record.Records)[0]
	}
	if oldValue == value && record.Ttl != nil && *record.Ttl == ttl {
		fmt.Printf("华为云 DNS 记录未变化，跳过: %s -> %s\n", fullName, value)
		return false, nil
	}
	if record.Id == nil || *record.Id == "" {
		return true, fmt.Errorf("华为云 DNS 记录 ID 为空: %s %s", fullName, recordType)
	}
	fmt.Printf("华为云 DNS 记录存在，更新: %s %s -> %s\n", fullName, oldValue, value)
	req := &dnsmodel.UpdateRecordSetRequest{
		ZoneId:      zoneID,
		RecordsetId: *record.Id,
		Body: &dnsmodel.UpdateRecordSetReq{
			Name:    tea.String(fqdn(fullName)),
			Type:    tea.String(recordType),
			Records: &records,
			Ttl:     tea.Int32(ttl),
		},
	}
	_, err = client.UpdateRecordSet(req)
	return true, err
}

func getHuaweiCDNDomain(client *cdn.CdnClient, domainName string) (*cdnmodel.Domains, error) {
	domainName = strings.TrimSpace(domainName)
	if domainName == "" {
		return nil, fmt.Errorf("华为 CDN 域名为空")
	}
	req := &cdnmodel.ListDomainsRequest{
		DomainName: &domainName,
	}

	resp, err := client.ListDomains(req)
	if err != nil {
		return nil, err
	}
	if resp.Domains == nil || len(*resp.Domains) == 0 {
		return nil, fmt.Errorf("%w: %s", errHuaweiCDNDomainNotFound, domainName)
	}
	for _, item := range *resp.Domains {
		if item.DomainName != nil && *item.DomainName == domainName {
			domain := item
			return &domain, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", errHuaweiCDNDomainNotFound, domainName)
}

func getHuaweiCDNDomainID(client *cdn.CdnClient, domainName string) (string, error) {
	domain, err := getHuaweiCDNDomain(client, domainName)
	if err != nil {
		return "", err
	}
	if domain.Id != nil && *domain.Id != "" {
		return *domain.Id, nil
	}
	return "", fmt.Errorf("未找到华为云 CDN 域名 ID: %s", domainName)
}
func getHuaweiCDNCname(client *cdn.CdnClient, domainName string) (string, error) {
	domain, err := getHuaweiCDNDomain(client, domainName)
	if err != nil {
		return "", err
	}
	if domain.Cname != nil && *domain.Cname != "" {
		return *domain.Cname, nil
	}
	return "", fmt.Errorf("未找到华为 CDN CNAME: %s", domainName)
}

func beforeCreateHuaweiCDNDomain(dns DNSRecord) error {
	client, err := newHuaweiCDNClient(dns.Key, dns.Secret)
	if err != nil {
		return err
	}

	infoReq := &cdnmodel.ShowVerifyDomainOwnerInfoRequest{
		DomainName: dns.Domain,
	}

	infoResp, err := client.ShowVerifyDomainOwnerInfo(infoReq)
	if err != nil {
		return err
	}

	if infoResp.VerifyDomainName == nil ||
		infoResp.DnsVerifyName == nil ||
		infoResp.VerifyContent == nil {
		return fmt.Errorf("华为 CDN 域名验证信息为空")
	}

	fmt.Println("验证域名:", *infoResp.VerifyDomainName)
	fmt.Println("TXT RR:", *infoResp.DnsVerifyName)
	fmt.Println("TXT Value:", *infoResp.VerifyContent)

	recordType := "TXT"
	if infoResp.DnsVerifyType != nil && *infoResp.DnsVerifyType != "" {
		recordType = *infoResp.DnsVerifyType
	}
	dns.RootDomain = *infoResp.VerifyDomainName
	dns.RR = *infoResp.DnsVerifyName
	dns.RecordType = strings.ToUpper(recordType)
	dns.Content = *infoResp.VerifyContent
	dns.TTL = 600

	flag, err := DnsUpsertRecord(dns)
	if err != nil {
		return err
	}

	if flag {
		if err := WaitTXT(dns.RR+"."+dns.RootDomain, dns.Content, 5*time.Minute); err != nil {
			return err
		}
	}
	verifyType := "dns"
	verifyReq := &cdnmodel.VerifyDomainOwnerRequest{
		DomainName: dns.Domain,
		Body: &cdnmodel.VerifyDomainOwnerRequestBody{
			VerifyType: &verifyType,
		},
	}

	_, err = client.VerifyDomainOwner(verifyReq)
	return err
}

func huaweiCDNOriginPorts(port int32) (int32, int32) {
	if port <= 0 {
		return 80, 443
	}
	if port == 80 {
		return 80, 443
	}
	if port == 443 {
		return 80, 443
	}
	return port, port
}

func huaweiCDNOriginType(dns DNSRecord) cdnmodel.SourcesRequestBodyOriginType {
	if dns.IsDomain {
		return cdnmodel.GetSourcesRequestBodyOriginTypeEnum().DOMAIN
	}
	return cdnmodel.GetSourcesRequestBodyOriginTypeEnum().IPADDR
}

func huaweiCDNOriginTypeValue(isDomain bool) string {
	if isDomain {
		return "domain"
	}
	return "ipaddr"
}

func huaweiCDNServiceArea(scope int) cdnmodel.DomainBodyServiceArea {
	switch scope {
	case 1:
		return cdnmodel.GetDomainBodyServiceAreaEnum().GLOBAL
	case 2:
		return cdnmodel.GetDomainBodyServiceAreaEnum().MAINLAND_CHINA
	default:
		return cdnmodel.GetDomainBodyServiceAreaEnum().OUTSIDE_MAINLAND_CHINA
	}
}
func updateHuaweiCDNConfig(client *cdn.CdnClient, dns DNSRecord) error {
	originProtocol := "http"
	if dns.IsDomain {
		originProtocol = "https"
	}
	zipType := tea.String(".html,.ts,.7z,.avi,.avif,.apk,.bin,.bmp,.bz2,.class,.css,.csv,.doc,.docx,.dmg,.ejs,.eot,.eps,.exe,.flac,.gif,.gz,.ico,.iso,.jar,.jpg,.jpeg,.js,.mid,.midi,.mkv,.mp3,.mp4,.ogg,.otf,.pdf,.pict,.pls,.png,.ppt,.pptx,.ps,.rar,.svg,.svgz,.swf,.tar,.tif,.tiff,.ttf,.webm,.webp,.woff,.woff2,.xls,.xlsx,.zip,.zst")
	req := &cdnmodel.UpdateDomainFullConfigRequest{
		DomainName: dns.Domain,
		Body: &cdnmodel.ModifyDomainConfigRequestBody{
			Configs: &cdnmodel.Configs{
				// 回源方式
				OriginProtocol: &originProtocol,
				// Range 回源
				OriginRangeStatus: tea.String("on"),
				// 回源跟随
				OriginFollow302Status: tea.String("on"),
				// 回源校验 ETag
				SliceEtagStatus:      tea.String("on"),
				OriginReceiveTimeout: tea.Int32(30),
				Sni: &cdnmodel.Sni{
					Status:     "on",
					ServerName: tea.String(dns.OriginDomain),
				},
				IpFrequencyLimit: &cdnmodel.IpFrequencyLimit{
					Status: "on",
					Qps:    tea.Int32(100),
				},
				// 智能压缩
				Compress: &cdnmodel.Compress{
					Status:   "on",
					Type:     tea.String("gzip"),
					FileType: zipType,
				},

				// 缓存规则
				CacheRules: &[]cdnmodel.CacheRules{
					{
						MatchType:  *tea.String("file_extension"),
						MatchValue: zipType,
						Ttl:        tea.Int32(86400),
						TtlUnit:    *tea.String("s"),
						ForceCache: tea.String("on"),
						Priority:   int32(2),
					},
					{
						MatchType:  *tea.String("all"),
						MatchValue: tea.String("*"),
						Ttl:        tea.Int32(86400),
						TtlUnit:    *tea.String("s"),
						ForceCache: tea.String("off"),
						Priority:   int32(1),
					},
				},
			},
		},
	}

	_, err := client.UpdateDomainFullConfig(req)
	return err
}
func createHuaweiCDNDomain(client *cdn.CdnClient, dns DNSRecord) error {
	httpPort, httpsPort := huaweiCDNOriginPorts(dns.Port)
	sources := []cdnmodel.SourcesRequestBody{
		{
			IpOrDomain:    dns.OriginDomain,
			OriginType:    huaweiCDNOriginType(dns),
			ActiveStandby: 1,
			HttpPort:      &httpPort,
			HttpsPort:     &httpsPort,
		},
	}

	serviceArea := huaweiCDNServiceArea(dns.Scope)
	businessType := cdnmodel.GetDomainBodyBusinessTypeEnum().WEB
	req := &cdnmodel.CreateDomainRequest{
		Body: &cdnmodel.CreateDomainRequestBody{
			Domain: &cdnmodel.DomainBody{
				DomainName:   dns.Domain,
				BusinessType: businessType,
				ServiceArea:  &serviceArea,
				Sources:      sources,
			},
		},
	}

	_, err := client.CreateDomain(req)
	if err != nil {
		return fmt.Errorf("create huawei cdn domain failed: %w", err)
	}
	time.Sleep(5 * time.Second)

	if err := updateHuaweiCDNConfig(client, dns); err != nil {
		return fmt.Errorf("update huawei cdn config failed: %w", err)
	}

	return nil
}
func updateHuaweiCDNOrigin(dns DNSRecord) (string, error) {
	client, err := newHuaweiCDNClient(dns.Key, dns.Secret)
	if err != nil {
		return "", err
	}
	_, err = getHuaweiCDNDomainID(client, dns.Domain)
	if err != nil {
		if !errors.Is(err, errHuaweiCDNDomainNotFound) {
			return "", err
		}
		fmt.Println("Huawei CDN 域名不存在，先验证归属")
		if err := beforeCreateHuaweiCDNDomain(dns); err != nil {
			return "", err
		}
		fmt.Println("归属验证成功，开始创建 CDN 域名")
		err = createHuaweiCDNDomain(client, dns)
		if err != nil {
			return "", err
		}
		time.Sleep(5 * time.Second)
		_, err = getHuaweiCDNDomainID(client, dns.Domain)
		if err != nil {
			return "", err
		}
	}
	httpPort, httpsPort := huaweiCDNOriginPorts(dns.Port)
	sourceWeight := int32(100)
	businessType := "web"
	req := &cdnmodel.UpdateDomainFullConfigRequest{
		DomainName: dns.Domain,
		Body: &cdnmodel.ModifyDomainConfigRequestBody{
			Configs: &cdnmodel.Configs{
				BusinessType: &businessType,
				Sources: &[]cdnmodel.SourcesConfig{
					{
						OriginAddr: dns.OriginDomain,
						HostName:   &dns.OriginDomain,
						OriginType: huaweiCDNOriginTypeValue(dns.IsDomain),
						Priority:   70,
						Weight:     &sourceWeight,
						HttpPort:   &httpPort,
						HttpsPort:  &httpsPort,
					},
				},
			},
		},
	}

	_, err = client.UpdateDomainFullConfig(req)
	if err != nil {
		return "", err
	}
	cname, err := getHuaweiCDNCname(client, dns.Domain)
	if err != nil {
		return "", err
	}
	return cname, nil
}
func UploadHuaweiCDNCert(dns DNSRecord, certName, certPem, keyPem string) error {
	client, err := newHuaweiCDNClient(dns.Key, dns.Secret)
	if err != nil {
		return err
	}
	httpsStatus := int32(1) // 1 开启 HTTPS
	certificateType := int32(0)
	req := &cdnmodel.UpdateDomainMultiCertificatesRequest{
		Body: &cdnmodel.UpdateDomainMultiCertificatesRequestBody{
			Https: &cdnmodel.UpdateDomainMultiCertificatesRequestBodyContent{
				HttpsSwitch:     httpsStatus,
				DomainName:      dns.Domain,
				CertName:        &certName,
				Certificate:     &certPem,
				PrivateKey:      &keyPem,
				CertificateType: &certificateType,
			},
		},
	}

	_, err = client.UpdateDomainMultiCertificates(req)
	return err
}
func PostHuaweiCdnDomain(dns DNSRecord) error {
	cname, err := updateHuaweiCDNOrigin(dns)
	if err != nil {
		fmt.Println("Failed to update Huawei CDN origin:", err)
		return err
	}
	if cname == "" {
		return fmt.Errorf("华为 CDN CNAME 为空: %s", dns.Domain)
	}

	dns.RR = dns.SubDomain
	dns.RecordType = "CNAME"
	dns.Content = cname
	dns.TTL = 600

	_, err = DnsUpsertRecord(dns)
	if err != nil {
		fmt.Println("Failed to upsert Huawei DNS CNAME record:", err)
		return err
	}

	return nil
}
