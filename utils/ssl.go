package utils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/go-acme/lego/challenge"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/alidns"
	"github.com/go-acme/lego/v4/providers/dns/baiducloud"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/huaweicloud"
	"github.com/go-acme/lego/v4/providers/dns/jdcloud"
	"github.com/go-acme/lego/v4/providers/dns/namesilo"
	"github.com/go-acme/lego/v4/providers/dns/tencentcloud"
	"github.com/go-acme/lego/v4/providers/dns/ucloud"
	"github.com/go-acme/lego/v4/registration"
)

type MyUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *MyUser) GetEmail() string {
	return u.Email
}

func (u *MyUser) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *MyUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

func newDNSProvider(t, key, secret string) (challenge.Provider, error) {
	switch t {
	case "ali":
		cfg := alidns.NewDefaultConfig()
		cfg.APIKey = key
		cfg.SecretKey = secret
		return alidns.NewDNSProviderConfig(cfg)
	case "tencent":
		cfg := tencentcloud.NewDefaultConfig()
		cfg.SecretID = key
		cfg.SecretKey = secret
		return tencentcloud.NewDNSProviderConfig(cfg)
	case "huawei":
		cfg := huaweicloud.NewDefaultConfig()
		cfg.AccessKeyID = key
		cfg.SecretAccessKey = secret
		return huaweicloud.NewDNSProviderConfig(cfg)
	case "baidu":
		cfg := baiducloud.NewDefaultConfig()
		cfg.AccessKeyID = key
		cfg.SecretAccessKey = secret
		return baiducloud.NewDNSProviderConfig(cfg)
	case "ucloud":
		cfg := ucloud.NewDefaultConfig()
		cfg.PublicKey = key
		cfg.PrivateKey = secret
		return ucloud.NewDNSProviderConfig(cfg)
	case "cloudflare":
		cfg := cloudflare.NewDefaultConfig()
		cfg.AuthToken = key
		return cloudflare.NewDNSProviderConfig(cfg)
	case "namesilo":
		cfg := namesilo.NewDefaultConfig()
		cfg.APIKey = key
		return namesilo.NewDNSProviderConfig(cfg)
	case "jdcloud":
		cfg := jdcloud.NewDefaultConfig()
		cfg.AccessKeyID = key
		cfg.AccessKeySecret = secret
		return jdcloud.NewDNSProviderConfig(cfg)
	}

	return nil, fmt.Errorf("unsupported provider")
}
func GetCertificate(t, domain, email, id, secret string) (*certificate.Resource, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	user := &MyUser{
		Email: email,
		key:   privateKey,
	}

	config := lego.NewConfig(user)
	config.Certificate.KeyType = certcrypto.RSA2048

	client, err := lego.NewClient(config)
	if err != nil {
		return nil, err
	}

	provider, err := newDNSProvider(t, id, secret)
	if err != nil {
		return nil, err
	}

	err = client.Challenge.SetDNS01Provider(
		provider,
		dns01.AddDNSTimeout(
			60*time.Second,
		),
	)
	if err != nil {
		return nil, err
	}
	reg, err := client.Registration.Register(registration.RegisterOptions{
		TermsOfServiceAgreed: true,
	})
	if err != nil {
		return nil, err
	}
	user.Registration = reg

	req := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(req)
	if err != nil {
		return nil, err
	}
	return certificates, nil
}
func GetCertExpireTime(certPEM []byte) (time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, err
	}

	return cert.NotAfter, nil
}
func UploadCDNCert(dns DNSRecord) (time.Time, error) {
	cert, err := GetCertificate(dns.T, dns.Domain, dns.ParentEmail, dns.ParentKey, dns.ParentSecret)
	if err != nil {
		if strings.Contains(err.Error(), "already issued for this exact set of identifiers in the last") {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("无法获取证书:%s", err)
	}
	switch dns.Tc {
	case "ali":
		certName := strings.ReplaceAll(dns.Domain, ".", "_") + fmt.Sprintf("-%d%02d%02d", dns.NowTime.Year(), dns.NowTime.Month(), dns.NowTime.Day())
		err := UploadAliCDNCert(dns.Key, dns.Secret, dns.Domain, certName, string(cert.Certificate), string(cert.PrivateKey))
		if err != nil {
			if strings.Contains(err.Error(), "CertNameAlreadyExists") {
				return time.Time{}, nil
			}
			return time.Time{}, fmt.Errorf("无法将证书上传到CDN:%s", err)
		}
	case "tencent":
		certName := strings.ReplaceAll(dns.Domain, ".", "_") + "_letsencrypt"
		err := UploadTencentCDNCert(dns, certName, string(cert.Certificate), string(cert.PrivateKey))
		if err != nil {
			return time.Time{}, fmt.Errorf("无法将证书上传到CDN:%s", err)
		}
	case "huawei":
		certName := strings.ReplaceAll(dns.Domain, ".", "_")
		err := UploadHuaweiCDNCert(dns, certName, string(cert.Certificate), string(cert.PrivateKey))
		if err != nil {
			return time.Time{}, fmt.Errorf("无法将证书上传到CDN:%s", err)
		}
	case "baidu":
		return time.Time{}, fmt.Errorf("百度云DNS不支持创建证书")
	case "ucloud":
		certName := strings.ReplaceAll(dns.Domain, ".", "_") + "_letsencrypt"
		err := UploadUCloudCDNCert(dns, certName, string(cert.Certificate), string(cert.PrivateKey))
		if err != nil {
			return time.Time{}, fmt.Errorf("无法将证书上传到CDN:%s", err)
		}
	case "namesilo":
		return time.Time{}, fmt.Errorf("namesilo云DNS不支持创建证书")
	case "jdcloud":
		return time.Time{}, fmt.Errorf("京东云DNS不支持创建证书")
	}

	expireTime, err := GetCertExpireTime(cert.Certificate)
	if err != nil {
		return time.Time{}, err
	}
	return expireTime, nil
}
