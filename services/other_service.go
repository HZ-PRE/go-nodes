package services

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"nodes/config"
	"nodes/models"
	"nodes/models/vo"
	"nodes/utils"
)

func (s *service) GetServerDictsByType(t uint8) ([]models.ServerDict, error) {
	return s.repo.GetServerDictsByType(t)
}
func (s *service) ServerDictsDel(id string) error {
	return s.repo.ServerDictsDel(id)
}
func (s *service) PostServerDicts(d models.ServerDict) error {
	if d.ID == "" && utils.Node != nil {
		d.ID = utils.Node.Generate().String()
	}
	return s.repo.PostServerDicts(d)
}
func (s *service) GetServerSupplierApi() ([]models.ServerSupplierApi, error) {
	return s.repo.GetServerSupplierApi()
}
func (s *service) GetServerSupplierApiBySupplier(supplier string, cdn int) ([]models.ServerSupplierApi, error) {
	return s.repo.GetServerSupplierApiBySupplier(supplier, cdn)
}
func (s *service) ServerSupplierApiDel(id int) error {
	return s.repo.ServerSupplierApiDel(id)
}
func (s *service) PostServerSupplierApi(api models.ServerSupplierApi) error {
	return s.repo.PostServerSupplierApi(api)
}
func (s *service) GetServerUrl() ([]models.ServerUrl, error) {
	return s.repo.GetServerUrl()
}
func (s *service) ServerUrlDel(id string) error {
	return s.repo.ServerUrlDel(id)
}
func (s *service) PostServerUrl(urls models.ServerUrl) error {
	return s.repo.PostServerUrl(urls)
}
func (s *service) GetServerHost() ([]models.ServerHost, error) {
	return s.repo.GetServerHost()
}
func (s *service) GetServerHostByApp(app string) ([]models.ServerHost, error) {
	return s.repo.GetServerHostByApp(app)
}
func (s *service) ServerHostDel(id string) error {
	return s.repo.ServerHostDel(id)
}
func (s *service) PostServerHost(hosts models.ServerHost) error {
	return s.repo.PostServerHost(hosts)
}
func (s *service) textServerHostBySslAt() error {
	suppliers, err := s.repo.GetServerHostBySslAt(1)
	if err != nil {
		return err
	}
	var msgBuilder strings.Builder
	for _, supplier := range suppliers {
		msgBuilder.WriteString(supplier.Domain)
		msgBuilder.WriteString("\n")
	}

	cfg := config.AppConfig
	msg := msgBuilder.String()
	if msg != "" {
		if cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
			msg += "\n以上域名多次证书申请失败，请及时处理"
			if err := utils.SendTelegramMessage(cfg.Telegram.BotToken, cfg.Telegram.ChatID, msg); err != nil {
				log.Printf("failed to send Telegram message: %v", err)
			}
		}
		return nil
	}
	return nil
}
func (s *service) uploadCDNDomainCert() error {
	suppliers, err := s.repo.GetServerHostBySslAt(10)
	if err != nil {
		return err
	}
	parentIds := make([]int, 0)
	for _, supplier := range suppliers {
		parentIds = append(parentIds, supplier.ParentID)
	}
	if len(parentIds) == 0 {
		return fmt.Errorf("未找到匹配的父供应商")
	}

	parentSuppliers, err := s.repo.GetServerHostByParentId(parentIds)
	if err != nil {
		return err
	}
	parentMap := make(map[int]vo.ServerHostVo)
	for _, parentSupplier := range parentSuppliers {
		parentMap[parentSupplier.ParentID] = parentSupplier
	}
	cfg := config.AppConfig
	now := time.Now()
	email := "canmaxsend@gmail.com"
	if cfg != nil && cfg.Server.Email != "" {
		email = cfg.Server.Email
	}
	dns := utils.DNSRecord{}

	for _, supplier := range suppliers {
		parentSupplier := parentMap[supplier.ParentID]
		if parentSupplier.ID == 0 {
			log.Printf("未找到匹配的父供应商:%s", supplier.Domain)
			continue
		}
		rootDomain := utils.GetRootDomain(supplier.Domain)
		if rootDomain == "" {
			log.Printf("获取根域名失败:%s", supplier.Domain)
			continue
		}
		originDomain := supplier.OriginDomain
		if strings.Contains(supplier.OriginDomain, ":") {
			h, _, err := net.SplitHostPort(supplier.OriginDomain)
			if err == nil {
				originDomain = h
			}
		}
		dns = utils.DNSRecord{
			T:            parentSupplier.Supplier,
			Tc:           supplier.Supplier,
			ParentKey:    parentSupplier.Key,
			ParentSecret: parentSupplier.Secret,
			Key:          supplier.Key,
			Secret:       supplier.Secret,
			Domain:       supplier.Domain,
			RootDomain:   rootDomain,
			SubDomain:    strings.Replace(supplier.Domain, "."+rootDomain, "", 1),
			TTL:          600,
			OriginDomain: originDomain,
			Scope:        supplier.Scope,
			Port:         443,
			IsDomain:     utils.IsDomain(originDomain),
			ParentEmail:  email,
			NowTime:      now,
		}

		if utils.IsEmail(parentSupplier.SupplierAccount) {
			dns.ParentEmail = parentSupplier.SupplierAccount
		}
		switch supplier.Supplier {
		case "ali":
			err = utils.PostAliCdnDomain(dns)
			if err != nil {
				log.Printf("%s 阿里 CDN 源站创建更新失败:%s", supplier.Domain, err)
				continue
			}
		case "tencent":
			err = utils.PostTencentCDNAndDNS(dns)
			if err != nil {
				log.Printf("%s 腾讯 CDN 源站创建更新失败:%s", supplier.Domain, err)
				continue
			}
		case "huawei":
			err = utils.PostHuaweiCdnDomain(dns)
			if err != nil {
				log.Printf("%s 华为 CDN 源站创建更新失败:%s", supplier.Domain, err)
				continue
			}
		case "baidu":
			continue
		case "ucloud":
			continue
		case "cloudflare":
			continue
		case "namesilo":
			continue
		case "jdcloud":
			continue
		}
		time, err := utils.UploadCDNCert(dns)
		if err != nil {
			log.Printf("%s 上传证书失败:%s", supplier.Domain, err)
			continue
		}
		if !time.IsZero() {
			s.repo.PutServerHostById(supplier.ID, time)
		}
	}
	return nil
}
func (s *service) PostServerDomain(hosts models.ServerHost) error {
	if 0 == hosts.SupplierId {
		return fmt.Errorf("请选择供应商")
	}
	parentSupplier, err := s.repo.GetServerSupplierApiByIdV1(hosts.ParentID)
	if err != nil || parentSupplier.ID == 0 {
		return fmt.Errorf("父供应商不存在")
	}
	supplier, err := s.repo.GetServerSupplierApiById(hosts.SupplierId)
	if err != nil {
		return fmt.Errorf("子供应商不存在")
	}
	cfg := config.AppConfig

	email := parentSupplier.SupplierAccount
	if !utils.IsEmail(parentSupplier.SupplierAccount) {
		if cfg != nil && cfg.Server.Email != "" {
			email = cfg.Server.Email
		} else {
			email = "canmaxsend@gmail.com"
		}
	}
	rootDomain := utils.GetRootDomain(hosts.Domain)
	if rootDomain == "" {
		return fmt.Errorf("获取根域名失败")
	}
	originPort := 443
	originDomain := hosts.OriginDomain
	if strings.Contains(hosts.OriginDomain, ":") {
		h, portStr, err := net.SplitHostPort(hosts.OriginDomain)
		if err == nil {
			originDomain = h
			originPort, err = strconv.Atoi(portStr)
			if err != nil {
				return fmt.Errorf("获取源站端口失败: %w", err)
			}
		}
	}
	now := time.Now()
	dns := utils.DNSRecord{
		T:            parentSupplier.Supplier,
		Tc:           supplier.Supplier,
		ParentKey:    parentSupplier.Key,
		ParentSecret: parentSupplier.Secret,
		Key:          supplier.Key,
		Secret:       supplier.Secret,
		Domain:       hosts.Domain,
		RootDomain:   rootDomain,
		SubDomain:    strings.Replace(hosts.Domain, "."+rootDomain, "", 1),
		TTL:          600,
		OriginDomain: originDomain,
		Scope:        hosts.Scope,
		Port:         443,
		IsDomain:     utils.IsDomain(originDomain),
		ParentEmail:  email,
		NowTime:      now,
	}
	isSsl := true
	var retErr error
	switch supplier.Supplier {
	case "ali":
		err = utils.PostAliCdnDomain(dns)
		if err != nil {
			return fmt.Errorf("阿里 CDN 源站创建更新失败:%s", err)
		}
	case "tencent":
		err = utils.PostTencentCDNAndDNS(dns)
		if err != nil {
			return fmt.Errorf("腾讯 CDN 源站创建更新失败:%s", err)
		}
	case "huawei":
		err = utils.PostHuaweiCdnDomain(dns)
		if err != nil {
			return fmt.Errorf("华为 CDN 源站创建更新失败:%s", err)
		}
	case "baidu":
		return fmt.Errorf("百度云DNS不支持API添加TXT记录，无法验证CDN域名所有者")
	case "ucloud":
		// err := utils.PostUCloudCDNAndDNS(dns)
		// if err != nil {
		// 	return fmt.Errorf("UCloud CDN 源站创建更新失败:%s", err)
		// }
		return fmt.Errorf("ucloudDNS不支持API添加TXT记录，无法验证CDN域名所有者")
	case "cloudflare":
		if strings.Contains(dns.SubDomain, ".") {
			return fmt.Errorf("cloudflare 域名代理不支持添加二级以上的域名，请使用二级域名")
		}
		dns.T = supplier.Supplier
		if dns.IsDomain {
			dns.RecordType = "CNAME"
		} else {
			dns.RecordType = "TXT"
		}
		dns.Proxied = true
		dns.RR = dns.SubDomain
		dns.Content = dns.OriginDomain
		_, err := utils.DnsUpsertRecord(dns)
		if err != nil {
			return fmt.Errorf("cloudflare 域名代理创建更新失败:%s", err)
		}
		err = utils.SetCFCacheRule(dns.ParentKey, rootDomain)
		if err != nil {
			return fmt.Errorf("cloudflare gateway规则创建失败:%s", err)
		}
		err = utils.SetCFRateLimitRule(dns.ParentKey, rootDomain)
		if err != nil {
			return fmt.Errorf("cloudflare 速率限制规则创建失败:%s", err)
		}
		err = utils.SetCFSSLMode(dns.ParentKey, rootDomain, "flexible")
		if err != nil {
			return fmt.Errorf("cloudflare SSL模式配置失败:%s", err)
		}
		if 443 != originPort && 80 != originPort {
			err = utils.SetCFOriginPortForSubdomain(dns.ParentKey, rootDomain, dns.SubDomain, int32(originPort))
			if err != nil {
				return fmt.Errorf("访问源站端口（%d）配置失败:%s", originPort, err)
			}
		}
		err = utils.SetCFOriginHostHeaderForSubdomain(dns.ParentKey, rootDomain, dns.SubDomain, dns.OriginDomain)
		if err != nil {
			retErr = fmt.Errorf("此错误可忽略,（域名已经成功代理）cloudflare origin host header配置失败(套餐权限不足，仅Enterprise套餐提供。升级计划来启用此功能)，请手动去源服务器配置添加代理域名:%s", err)
		}
		isSsl = false
	case "namesilo":
		return fmt.Errorf("namesilo云DNS不支持API添加TXT记录，无法验证CDN域名所有者")
	case "jdcloud":
		return fmt.Errorf("京东云DNS不支持API添加TXT记录，无法验证CDN域名所有者")
	}
	if isSsl {
		time, err := utils.UploadCDNCert(dns)
		if err != nil {
			return err
		}
		if time.IsZero() {
			time = now.AddDate(0, 0, 85)
		}
		hosts.SslAt = time
	}
	err = s.repo.PostServerHost(hosts)
	if err != nil {
		return err
	}
	return retErr
}
func (s *service) PutServerHostByDomain(domain []string) error {
	return s.repo.PutServerHostByDomain(domain)
}
func (s *service) ApiNelNet() error {
	cfg := config.AppConfig
	if cfg == nil {
		return nil
	}

	urls, err := s.repo.GetServerUrl()
	if err != nil {
		return err
	}
	if len(urls) == 0 {
		return nil
	}

	type probeResult struct {
		note    string
		id      string
		success bool
	}

	maxWorkers := utils.ProbeWorkers()
	sem := make(chan struct{}, maxWorkers)
	ch := make(chan probeResult, len(urls))

	for _, item := range urls {
		sem <- struct{}{}
		go func(id, note, url string) {
			ok, probeErr := utils.YCIpNelNet(url)
			ch <- probeResult{
				note:    fmt.Sprintf("%s: (err=%v)", note, probeErr),
				id:      id,
				success: ok,
			}
			<-sem
		}(item.ID, item.Note, item.Url)
	}

	var msgBuilder strings.Builder
	hasMsg := false
	tempErrMap := make(map[string]uint)
	for range urls {
		r := <-ch
		shouldAppend := false
		if !r.success {
			if _, exists := errUrlMap[r.id]; !exists {
				tempErrMap[r.id] = 1
			} else {
				tempErrMap[r.id] = errUrlMap[r.id]
				tempErrMap[r.id]++
				if tempErrMap[r.id] > 3 {
					shouldAppend = true
				}
			}
		}

		if shouldAppend {
			if hasMsg {
				msgBuilder.WriteByte('\n')
			}
			msgBuilder.WriteString(r.note)
			hasMsg = true
		}
	}
	errIpMu.Lock()
	errUrlMap = tempErrMap
	errIpMu.Unlock()
	msg := msgBuilder.String()
	if msg != "" && cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
		msg += "\n以上接口不可访问，请及时处理"
		if err := utils.SendTelegramMessage(cfg.Telegram.BotToken, cfg.Telegram.ChatID, msg); err != nil {
			log.Printf("failed to send Telegram message: %v", err)
		}
	}
	return nil
}

func YCIpBC(errIpArr map[string]string, isSend bool) bool {
	cfg := config.AppConfig
	if cfg == nil || len(cfg.Node.URL) == 0 || len(errIpArr) == 0 {
		return false
	}

	type probeResult struct {
		name    string
		ip      string
		success bool
	}

	maxWorkers := utils.ProbeWorkers()
	sem := make(chan struct{}, maxWorkers)
	ch := make(chan probeResult, len(errIpArr))

	for ip, name := range errIpArr {
		parts := strings.SplitN(ip, ":", 2)
		if len(parts) != 2 {
			// Treat malformed endpoint as skipped (not failed).
			ch <- probeResult{ip: ip, name: name, success: true}
			continue
		}

		host, port := parts[0], parts[1]
		sem <- struct{}{}
		go func(ip, host, port, name string) {
			ok := false
			for _, u := range cfg.Node.URL {
				if r, _ := utils.YCIpNelNet(fmt.Sprintf("%s/%s/%s", u, host, port)); r {
					ok = true
					break
				}
			}
			ch <- probeResult{ip: ip, name: name, success: ok}
			<-sem
		}(ip, host, port, name)
	}

	var msgBuilder strings.Builder
	hasMsg := false
	tempErrIpMap := make(map[string]uint)
	for range errIpArr {
		r := <-ch
		shouldAppend := false

		if !r.success {
			errIpMu.Lock()
			errIpMap[r.ip]++
			tempErrIpMap[r.ip] = errIpMap[r.ip]
			if !isSend || errIpMap[r.ip] > 3 {
				shouldAppend = true
			}
			errIpMu.Unlock()
		}

		if shouldAppend {
			if hasMsg {
				msgBuilder.WriteByte('\n')
			}
			msgBuilder.WriteString(r.name)
			msgBuilder.WriteString(" (")
			msgBuilder.WriteString(r.ip)
			msgBuilder.WriteString(")")
			hasMsg = true
		}
	}

	msg := msgBuilder.String()
	if msg != "" {
		errIpMu.Lock()
		errIpMap = tempErrIpMap
		errIpMu.Unlock()
		if isSend && cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
			msg += "\n以上节点故障，请及时处理"
			if err := utils.SendTelegramMessage(cfg.Telegram.BotToken, cfg.Telegram.ChatID, msg); err != nil {
				log.Printf("failed to send Telegram message: %v", err)
			}
		}
		return false
	}
	return true
}
