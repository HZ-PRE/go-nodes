package utils

import (
	"fmt"
	"net"
	"time"
)

type DNSRecord struct {
	T            string
	Tc           string
	ParentKey    string
	ParentSecret string
	Key          string
	Secret       string
	RootDomain   string
	SubDomain    string
	Domain       string
	OriginDomain string
	Content      string
	RecordType   string
	RR           string
	TTL          int64
	Proxied      bool
	Scope        int
	Port         int32
	IsDomain     bool
	ParentEmail  string
	NowTime      time.Time
	ProjectID    string
	Region       string
}

func WaitTXT(fqdn, value string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		txts, _ := net.LookupTXT(fqdn)
		for _, txt := range txts {
			if txt == value {
				return nil
			}
		}
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("TXT 未生效: %s = %s", fqdn, value)
}
func DnsUpsertRecord(d DNSRecord) (bool, error) {
	switch d.T {
	case "ali":
		return AliUpsertRecord(d.ParentKey, d.ParentSecret, d.RootDomain, d.RR, d.RecordType, d.Content, d.TTL)
	case "cloudflare":
		return UpsertCFRecord(d.ParentKey, d.RootDomain, d.RR, d.RecordType, d.Content, d.TTL, d.Proxied)
	case "huawei":
		rr := d.RR
		if rr == "" {
			rr = d.SubDomain
		}
		return UpsertHuaweiDNSRecord(d.ParentKey, d.ParentSecret, d.RootDomain, rr, d.RecordType, d.Content, int32(d.TTL))
	case "ucloud":
		return UpsertUCloudDNSRecord(d.ParentKey, d.ParentSecret, d.RootDomain, d.RR, d.RecordType, d.Content, d.ProjectID, d.Region, d.TTL)
	case "tencent":
		return UpsertTencentDNSRecord(d.ParentKey, d.ParentSecret, d.RootDomain, d.RR, d.RecordType, d.Content, uint64(d.TTL))
	}
	return true, nil
}
