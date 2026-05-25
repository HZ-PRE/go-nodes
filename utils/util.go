package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var base36Table [256]uint8

func init() {
	for i := byte('0'); i <= '9'; i++ {
		base36Table[i] = i - '0'
	}
	for i := byte('a'); i <= 'z'; i++ {
		base36Table[i] = i - 'a' + 10
	}
	for i := byte('A'); i <= 'Z'; i++ {
		base36Table[i] = i - 'A' + 10
	}
}

func ParseBase36Fast2(s string) uint64 {
	var n uint64
	for i := 0; i < len(s); i++ {
		n = n*36 + uint64(base36Table[s[i]])
	}
	return n
}

// 判断是否是邮箱
func IsEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

var domainRegexp = regexp.MustCompile(
	`^(?i)[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`,
)

// 判断是否是域名
func IsDomain(domain string) bool {
	if len(domain) > 253 {
		return false
	}
	return domainRegexp.MatchString(domain)
}

// GetRootDomain 从一个域名中提取根域名，例如 "www.example.com" -> "example.com"。如果解析失败，则返回原始域名。
func GetRootDomain(domain string) string {
	root, err := publicsuffix.EffectiveTLDPlusOne(domain)
	if err != nil {
		return domain
	}
	return root
}

// 根据可用的CPU核心数返回一个工作线程数量，范围在4到NumCPU之间。如果NumCPU小于4，则返回4。
func ProbeWorkers() int {
	n := runtime.NumCPU()
	if n < 4 {
		n = 4
	}
	return n
}
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func GzipCompressBase64(input []byte) (string, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(input); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func IpNelNet(ip, port string) (bool, error) {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), timeout)
	if err != nil {
		return false, err
	}
	_ = conn.Close()
	return true, nil
}
func IpToUint32(ip string) uint32 {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return 0
	}
	a, _ := strconv.Atoi(parts[0])
	b, _ := strconv.Atoi(parts[1])
	c, _ := strconv.Atoi(parts[2])
	d, _ := strconv.Atoi(parts[3])
	return uint32(a)<<24 |
		uint32(b)<<16 |
		uint32(c)<<8 |
		uint32(d)
}
func SendTelegramMessage(botToken, chatID, message string) error {
	if botToken == "" || chatID == "" || message == "" {
		return fmt.Errorf("telegram params required")
	}

	api := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}{
		ChatID: chatID,
		Text:   message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, api, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := telegramHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram status=%d body=%s", resp.StatusCode, string(raw))
	}
	return nil
}

var httpClient = &http.Client{Timeout: 5 * time.Second}
var telegramHTTPClient = &http.Client{Timeout: 10 * time.Second}

func YCIpNelNet(url string) (bool, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("url:%s status code: %d", url, resp.StatusCode)
	}
	return true, nil
}
func GetLinuxToDevice(host, user, password, keyPath string) (string, error) {
	cmd := `if command -v ip >/dev/null 2>&1; then ip route | awk '/default/ {print $5; exit}'; else route -n | awk '/^0.0.0.0/ {print $8; exit}'; fi`
	out, err := SshRunCommand(host, user, password, keyPath, cmd)
	if err != nil {
		fmt.Printf("获取%s驱动号失败: %v\n", host, err)
		return "", err
	}
	out = strings.TrimSpace(out)
	return out, nil
}

// keyPath 可选：私钥路径（如 ~/.ssh/id_rsa）
func SshRunCommand(host, user, password, keyPath, cmd string) (string, error) {
	var auth ssh.AuthMethod
	// ===== 判断认证方式 =====
	if password != "" && password != "0" {
		auth = ssh.Password(password) // 密码认证
	} else {
		key, err := os.ReadFile(keyPath) // 私钥认证
		if err != nil {
			return "", fmt.Errorf("read key error: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return "", fmt.Errorf("parse key error: %w", err)
		}
		auth = ssh.PublicKeys(signer)
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ⚠️ 生产建议校验
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	out, err := session.CombinedOutput(cmd)
	return string(out), err
}
func SshUploadContent(host, user, password, remoteFile, keyPath string, content []byte) error {
	var auth ssh.AuthMethod

	// 认证方式
	if password != "" && password != "0" {
		auth = ssh.Password(password)
	} else {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("read key error: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("parse key error: %w", err)
		}
		auth = ssh.PublicKeys(signer)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // ⚠️生产建议校验
		Timeout:         10 * time.Second,
	}

	// 建立 SSH 连接
	conn, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return err
	}
	defer conn.Close()

	// 创建 SFTP client
	client, err := sftp.NewClient(conn)
	if err != nil {
		return err
	}
	defer client.Close()

	// 👉 自动创建远程目录（关键点）
	dir := filepath.Dir(remoteFile)
	if err := client.MkdirAll(dir); err != nil {
		return fmt.Errorf("mkdir error: %w", err)
	}

	// 创建远程文件
	file, err := client.Create(remoteFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// 写入内容
	_, err = file.Write(content)
	return err
}

// 转小写，然后首字母大写
func ToTitleAdvanced(s string) string {
	caser := cases.Title(language.English)
	return caser.String(strings.ToLower(s))
}
