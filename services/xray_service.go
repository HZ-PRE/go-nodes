package services

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"nodes/utils"

	"nodes/models"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Log struct {
		Level      string `yaml:"Level"`
		AccessPath string `yaml:"AccessPath"`
		ErrorPath  string `yaml:"ErrorPath"`
	} `yaml:"Log"`
	DnsConfigPath      string `yaml:"DnsConfigPath"`
	RouteConfigPath    string `yaml:"RouteConfigPath"`
	InboundConfigPath  string `yaml:"InboundConfigPath"`
	OutboundConfigPath string `yaml:"OutboundConfigPath"`
	ConnectionConfig   struct {
		Handshake    int `yaml:"Handshake"`
		ConnIdle     int `yaml:"ConnIdle"`
		UplinkOnly   int `yaml:"UplinkOnly"`
		DownlinkOnly int `yaml:"DownlinkOnly"`
		BufferSize   int `yaml:"BufferSize"`
	} `yaml:"ConnectionConfig"`
	Nodes []Node `yaml:"Nodes"`
}
type Node struct {
	PanelType        string           `yaml:"PanelType"`
	ApiConfig        ApiConfig        `yaml:"ApiConfig"`
	ControllerConfig ControllerConfig `yaml:"ControllerConfig"`
}

type ApiConfig struct {
	ApiHost             string `yaml:"ApiHost"`
	ApiKey              string `yaml:"ApiKey"`
	NodeID              int    `yaml:"NodeID"`
	NodeType            string `yaml:"NodeType"`
	Timeout             int    `yaml:"Timeout"`
	EnableVless         bool   `yaml:"EnableVless"`
	VlessFlow           string `yaml:"VlessFlow"`
	SpeedLimit          int    `yaml:"SpeedLimit"`
	DeviceLimit         int    `yaml:"DeviceLimit"`
	RuleListPath        string `yaml:"RuleListPath"`
	DisableCustomConfig bool   `yaml:"DisableCustomConfig"`
}

type ControllerConfig struct {
	ListenIP            string `yaml:"ListenIP"`
	SendIP              string `yaml:"SendIP"`
	UpdatePeriodic      int    `yaml:"UpdatePeriodic"`
	EnableDNS           bool   `yaml:"EnableDNS"`
	DNSType             string `yaml:"DNSType"`
	EnableProxyProtocol bool   `yaml:"EnableProxyProtocol"`

	AutoSpeedLimitConfig      AutoSpeedLimitConfig    `yaml:"AutoSpeedLimitConfig"`
	GlobalDeviceLimitConfig   GlobalDeviceLimitConfig `yaml:"GlobalDeviceLimitConfig"`
	EnableFallback            bool                    `yaml:"EnableFallback"`
	FallBackConfigs           []FallBackConfig        `yaml:"FallBackConfigs"`
	DisableLocalREALITYConfig bool                    `yaml:"DisableLocalREALITYConfig"`
	EnableREALITY             bool                    `yaml:"EnableREALITY"`
	REALITYConfigs            REALITYConfig           `yaml:"REALITYConfigs"`
	CertConfig                CertConfig              `yaml:"CertConfig"`
}

type AutoSpeedLimitConfig struct {
	Limit         int `yaml:"Limit"`
	WarnTimes     int `yaml:"WarnTimes"`
	LimitSpeed    int `yaml:"LimitSpeed"`
	LimitDuration int `yaml:"LimitDuration"`
}

type GlobalDeviceLimitConfig struct {
	Enable        bool   `yaml:"Enable"`
	RedisNetwork  string `yaml:"RedisNetwork"`
	RedisAddr     string `yaml:"RedisAddr"`
	RedisUsername string `yaml:"RedisUsername"`
	RedisPassword string `yaml:"RedisPassword"`
	RedisDB       int    `yaml:"RedisDB"`
	Timeout       int    `yaml:"Timeout"`
	Expiry        int    `yaml:"Expiry"`
}

type FallBackConfig struct {
	SNI              string `yaml:"SNI"`
	Alpn             string `yaml:"Alpn"`
	Path             string `yaml:"Path"`
	Dest             int    `yaml:"Dest"`
	ProxyProtocolVer int    `yaml:"ProxyProtocolVer"`
}

type REALITYConfig struct {
	Show             bool     `yaml:"Show"`
	Dest             string   `yaml:"Dest"`
	ProxyProtocolVer int      `yaml:"ProxyProtocolVer"`
	ServerNames      []string `yaml:"ServerNames"`
	PrivateKey       string   `yaml:"PrivateKey"`
	MinClientVer     string   `yaml:"MinClientVer"`
	MaxClientVer     string   `yaml:"MaxClientVer"`
	MaxTimeDiff      int      `yaml:"MaxTimeDiff"`
	ShortIds         []string `yaml:"ShortIds"`
}

type CertConfig struct {
	CertMode   string            `yaml:"CertMode"`
	CertDomain string            `yaml:"CertDomain"`
	CertFile   string            `yaml:"CertFile"`
	KeyFile    string            `yaml:"KeyFile"`
	Provider   string            `yaml:"Provider"`
	Email      string            `yaml:"Email"`
	DNSEnv     map[string]string `yaml:"DNSEnv"`
}
type NodeInput struct {
	PanelType string
	ApiHost   string
	ApiKey    string
	NodeID    int
	NodeType  string

	EnableVless bool
	DeviceLimit int

	EnableFallback            bool
	DisableLocalREALITYConfig bool
	EnableReality             bool
	RealityDest               string
	RealityServer             string
	PrivateKey                string

	CertMode string
	Domain   string
}

type GostNode struct {
	Name       string // gost_yjl_dg_a
	LocalPort  uint   // 21013
	RemoteHost string // 80.240.17.122
	RemotePort uint   // 9904
}

func buildConfig(nodeInputs []NodeInput) ([]byte, error) {

	cfg := Config{}

	cfg.Log.Level = "error"
	cfg.Log.ErrorPath = "/etc/XrayR/error.log"

	cfg.ConnectionConfig = struct {
		Handshake    int `yaml:"Handshake"`
		ConnIdle     int `yaml:"ConnIdle"`
		UplinkOnly   int `yaml:"UplinkOnly"`
		DownlinkOnly int `yaml:"DownlinkOnly"`
		BufferSize   int `yaml:"BufferSize"`
	}{
		Handshake:    4,
		ConnIdle:     30,
		UplinkOnly:   2,
		DownlinkOnly: 4,
		BufferSize:   512,
	}

	// ===== 动态 Nodes =====
	for _, n := range nodeInputs {

		node := Node{
			PanelType: n.PanelType,
			ApiConfig: ApiConfig{
				ApiHost:     n.ApiHost,
				ApiKey:      n.ApiKey,
				NodeID:      n.NodeID,
				NodeType:    n.NodeType,
				Timeout:     30,
				EnableVless: n.EnableVless,
				VlessFlow:   "xtls-rprx-vision",
				SpeedLimit:  0,
				DeviceLimit: n.DeviceLimit,
			},
			ControllerConfig: ControllerConfig{
				ListenIP:       "0.0.0.0",
				SendIP:         "0.0.0.0",
				UpdatePeriodic: 60,
				DNSType:        "AsIs",

				EnableFallback: n.EnableFallback,

				FallBackConfigs: []FallBackConfig{
					{
						Dest: 80,
					},
				},
				DisableLocalREALITYConfig: n.DisableLocalREALITYConfig,
				EnableREALITY:             n.EnableReality,
				REALITYConfigs: REALITYConfig{
					Show:        true,
					Dest:        n.RealityDest,
					ServerNames: []string{n.RealityServer},
					PrivateKey:  n.PrivateKey,
					ShortIds:    []string{"", "0123456789abcdef"},
				},

				CertConfig: CertConfig{
					CertMode:   n.CertMode,
					CertDomain: n.Domain,
				},
			},
		}

		cfg.Nodes = append(cfg.Nodes, node)
	}

	return yaml.Marshal(cfg)
}

const gostTpl = `
{{- range . }}
[program:{{ .Name }}]
command=/gost/gost -L=tcp://:{{ .LocalPort }}/{{ .RemoteHost }}:{{ .RemotePort }} -L=udp://:{{ .LocalPort }}/{{ .RemoteHost }}:{{ .RemotePort }}
autorestart=true
directory=/gost/
stderr_logfile=/dev/null
stdout_logfile=/dev/null

{{ end }}
`
const wgetCmd = `
# 判断 wget 是否存在
if command -v wget >/dev/null 2>&1; then
    echo "wget 已安装: $(command -v wget)"
else
    echo "wget 未安装，开始安装..."

    # Debian / Ubuntu
    if command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update
        sudo apt-get install -y wget

    # CentOS / RHEL 7
    elif command -v yum >/dev/null 2>&1; then
        sudo yum install -y wget

    # RHEL / CentOS / Fedora 新版
    elif command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y wget

    # Alpine
    elif command -v apk >/dev/null 2>&1; then
        sudo apk add wget

    # Arch Linux
    elif command -v pacman >/dev/null 2>&1; then
        sudo pacman -Sy --noconfirm wget

    else
        echo "未识别的包管理器，请手动安装 wget"
        exit 1
    fi

    # 再次检查
    if command -v wget >/dev/null 2>&1; then
        echo "wget 安装成功"
    else
        echo "wget 安装失败"
        exit 1
    fi
fi
`
const supervisorCmd = `
# 判断 supervisorctl 是否存在
if command -v supervisorctl >/dev/null 2>&1; then
    echo "supervisor 已安装"
else
    echo "supervisor 未安装，开始安装..."

    # Debian / Ubuntu
    if command -v apt-get >/dev/null 2>&1; then
        apt-get update -y
        apt-get install -y supervisor

        systemctl enable supervisor 2>/dev/null || true
        systemctl restart supervisor 2>/dev/null || true

    # CentOS / RHEL 7
    elif command -v yum >/dev/null 2>&1; then
        yum install -y epel-release
        yum install -y supervisor

        systemctl enable supervisord 2>/dev/null || true
        systemctl restart supervisord 2>/dev/null || true

    # RHEL / Rocky / AlmaLinux / Fedora
    elif command -v dnf >/dev/null 2>&1; then
        dnf install -y supervisor

        systemctl enable supervisord 2>/dev/null || true
        systemctl restart supervisord 2>/dev/null || true

    # Alpine
    elif command -v apk >/dev/null 2>&1; then
        apk add supervisor

        rc-update add supervisord default 2>/dev/null || true
        service supervisord restart 2>/dev/null || true

    # Arch Linux
    elif command -v pacman >/dev/null 2>&1; then
        pacman -Sy --noconfirm supervisor

        systemctl enable supervisord 2>/dev/null || true
        systemctl restart supervisord 2>/dev/null || true

    else
        echo "未识别的包管理器，请手动安装 supervisor"
        exit 1
    fi

    # 再次检查
    if command -v supervisorctl >/dev/null 2>&1; then
        echo "supervisor 安装成功"
    else
        echo "supervisor 安装失败"
        exit 1
    fi
fi
`

func BuildGostConfig(nodes []GostNode) (string, error) {
	tpl, err := template.New("gost").Parse(gostTpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tpl.Execute(&buf, nodes)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
func (s *service) GostCon(id, app string) (string, error) {
	node, err := s.repo.GetServerNodesByIdV1(id)
	if err != nil {
		return "", err
	}
	if node.OutIP == "" {
		return "", fmt.Errorf("没有找到可部署的节点")
	}
	sNodes, err := s.repo.GetServersByApp(app)
	if len(sNodes) == 0 {
		return "", fmt.Errorf("没有找到应用 %s 的落地服务器", app)
	}
	nodes := make([]GostNode, len(sNodes))

	for i, n := range sNodes {
		if n.OutHost == "" {
			return "", fmt.Errorf("落地ip不能为空")
		}
		nodes[i] = GostNode{
			Name:       fmt.Sprintf("gost_%s_%d", app, n.NodeID),
			LocalPort:  n.Port,
			RemoteHost: n.OutHost,
			RemotePort: n.ServerPort,
		}
	}
	content, _ := BuildGostConfig(nodes)
	fmt.Println(content)
	url := fmt.Sprintf("%s:%d", node.OutIP, node.OutIPPort)
	if err != nil {
		fmt.Println("生成配置失败:", url, err)
		return "", err
	}
	config := []byte(content)
	err = utils.SshUploadContent(
		url,
		node.OutIPUser,
		node.OutIPPwd,
		fmt.Sprintf("/etc/supervisor/conf.d/%s_node.conf", app),
		"./conf/ssh_xrary_id_rsa",
		config,
	)

	if err != nil {
		fmt.Println("上传失败:", err)
		return "", err
	} else {
		fmt.Println("上传成功")
		if node.ZzApp == "" {
			node.ZzApp = app
		} else if !strings.Contains(node.ZzApp, app) {
			node.ZzApp = fmt.Sprintf("%s,%s", node.ZzApp, app)
		}
		s.repo.UpdateServersNodeById(id, models.ServerNode{ZzApp: node.ZzApp})
		out, err := utils.SshRunCommand(url, node.OutIPUser, node.OutIPPwd, "./conf/ssh_xrary_id_rsa", "supervisorctl reload && supervisorctl update")
		if err != nil {
			fmt.Println("加载失败:", err)
			return "", err
		}
		fmt.Println("加载成功:", out)
		return out, nil
	}
}

func (s *service) XrayInitCore(id string) (string, error) {
	sNodes, err := s.repo.GetServerNodesByIdV1(id)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s:%d", sNodes.OutIP, sNodes.OutIPPort)
	device, err := utils.GetLinuxToDevice(url, sNodes.OutIPUser, sNodes.OutIPPwd, "./conf/ssh_xrary_id_rsa")
	if err != nil {
		return "", err
	}
	cmd := fmt.Sprintf(`
nohup bash -c '
# ===== 关闭防火墙（如果存在） =====
if command -v systemctl >/dev/null 2>&1; then
    systemctl stop firewalld 2>/dev/null || true
    systemctl disable firewalld 2>/dev/null || true

    systemctl stop ufw 2>/dev/null || true
    systemctl disable ufw 2>/dev/null || true
fi

# iptables 清空（可选）
iptables -F 2>/dev/null || true
%s
# ===== 下载并执行脚本 =====
wget -q -N --no-check-certificate -O ./script.sh https://pub-eed78fedcfb6470ea94589a3771b4e0f.r2.dev/xray/script.sh && \
chmod 755 script.sh && \
bash ./script.sh
' > /tmp/XrayConInstall.log 2>&1 &

echo "started"
`, wgetCmd)
	out, err := utils.SshRunCommand(url, sNodes.OutIPUser, sNodes.OutIPPwd, "./conf/ssh_xrary_id_rsa", cmd)
	if err != nil {
		fmt.Println("Xray核心安装失败:", url, err)
		return "", err
	}
	fmt.Println("Xray核心安装成功:", out)
	err = s.repo.UpdateServersNodeById(id, models.ServerNode{Device: device, IsXray: 1})
	if err != nil {
		return "", fmt.Errorf("Xray核心安装成功:%s,更新设备信息失败:%s", out, err)
	}
	return out, nil
}
func (s *service) NodeExporterInit(id string) (string, error) {
	sNodes, err := s.repo.GetServerNodesByIdV1(id)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s:%d", sNodes.OutIP, sNodes.OutIPPort)
	device, err := utils.GetLinuxToDevice(url, sNodes.OutIPUser, sNodes.OutIPPwd, "./conf/ssh_xrary_id_rsa")
	if err != nil {
		return "", err
	}
	cmd := fmt.Sprintf(`
nohup bash -c '
# ===== 关闭防火墙（如果存在） =====
if command -v systemctl >/dev/null 2>&1; then
    systemctl stop firewalld 2>/dev/null || true
    systemctl disable firewalld 2>/dev/null || true

    systemctl stop ufw 2>/dev/null || true
    systemctl disable ufw 2>/dev/null || true
fi

# iptables 清空（可选）
iptables -F 2>/dev/null || true
%s
%s
tmp="/opt/node_exporter.tmp.$$"

wget -q --no-check-certificate -O "$tmp" \
  https://pub-eed78fedcfb6470ea94589a3771b4e0f.r2.dev/xray/node_exporter

chmod -R 777 /opt

supervisorctl stop node_exporter 2>/dev/null || true

mv -f "$tmp" /opt/node_exporter
supervisorctl start node_exporter 2>/dev/null || true
wget -q -N --no-check-certificate -O /etc/supervisor/conf.d/pub.conf https://pub-eed78fedcfb6470ea94589a3771b4e0f.r2.dev/xray/supervisor-pub.conf && supervisorctl reload && supervisorctl update 2>/dev/null || true
' > /tmp/NodeExporterInstall.log 2>&1 &
echo "started"
`, wgetCmd, supervisorCmd)
	out, err := utils.SshRunCommand(url, sNodes.OutIPUser, sNodes.OutIPPwd, "./conf/ssh_xrary_id_rsa", cmd)
	if err != nil {
		fmt.Println("节点监控安装失败:", url, err)
		return "", err
	}
	err = s.repo.UpdateServersNodeById(id, models.ServerNode{Device: device})
	if err != nil {
		return "", fmt.Errorf("节点监控安装成功:%s,更新设备信息失败:%s", out, err)
	}
	fmt.Println("节点监控安装成功:", out)
	return out, nil
}
func (s *service) XrayCon(id string) (string, error) {
	sNodes, err := s.repo.GetServerNodesById(id)
	if err != nil {
		return "", err
	}
	if len(sNodes) == 0 {
		return "", fmt.Errorf("没有找到可部署的节点")
	}
	ds, err := s.repo.GetServerDictsByType(1)
	if err != nil {
		return "", err
	}
	if len(ds) == 0 {
		return "", fmt.Errorf("没有找到对应的节点配置")
	}
	dsMap := make(map[string]models.ServerDict)
	for _, n := range ds {
		dsMap[string(n.Key)] = n
	}
	nodes := make([]NodeInput, len(sNodes))

	for i, n := range sNodes {
		if n.Method == "" {
			return "", fmt.Errorf("节点方法不能为空")
		}
		nodeType := strings.ToLower(n.Method)
		con, ok := dsMap[n.ZzApp]
		if !ok {
			return "", fmt.Errorf("没有找到对应的节点配置:%s", n.ZzApp)
		}
		if nodeType == "xrayv2ray" || nodeType == "xrayvless" {
			nodes[i] = NodeInput{
				PanelType:                 "NewV2board",
				ApiHost:                   con.Val,
				ApiKey:                    con.Note,
				NodeID:                    int(n.NodeID),
				NodeType:                  "V2ray",
				DeviceLimit:               4,
				EnableVless:               true,
				EnableFallback:            true,
				DisableLocalREALITYConfig: true,
				EnableReality:             true,
				RealityDest:               "www.apple.com:443",
				RealityServer:             "www.apple.com",
				PrivateKey:                "xxxx",
				CertMode:                  "none",
			}
		} else if nodeType == "xrayss" || nodeType == "xrayshadowsocks" {
			nodes[i] = NodeInput{
				PanelType:   "NewV2board",
				ApiHost:     con.Val,
				ApiKey:      con.Note,
				NodeID:      int(n.NodeID),
				NodeType:    "Shadowsocks",
				DeviceLimit: 4,
			}
		}
	}
	content, err := buildConfig(nodes)
	url := fmt.Sprintf("%s:%d", sNodes[0].OutIP, sNodes[0].OutIPPort)
	if err != nil {
		fmt.Println("生成配置失败:", url, err)
		return "", err
	}
	config := []byte(content)

	err = utils.SshUploadContent(
		url,
		sNodes[0].OutIPUser,
		sNodes[0].OutIPPwd,
		"/etc/XrayR/config.yml",
		"./conf/ssh_xrary_id_rsa",
		config,
	)

	if err != nil {
		fmt.Println("上传失败:", err)
		return "", err
	} else {
		fmt.Println("上传成功")
		out, err := utils.SshRunCommand(url, sNodes[0].OutIPUser, sNodes[0].OutIPPwd, "./conf/ssh_xrary_id_rsa", "XrayR restart")
		if err != nil {
			fmt.Println("重启失败:", err)
			return "", err
		}
		fmt.Println("重启成功:", out)
		return out, nil
	}
}
func (s *service) LinuxCMD(id, cmd string) (string, error) {
	sNodes, err := s.repo.GetServerNodesByIdV1(id)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s:%d", sNodes.OutIP, sNodes.OutIPPort)
	out, err := utils.SshRunCommand(url, sNodes.OutIPUser, sNodes.OutIPPwd, "./conf/ssh_xrary_id_rsa", cmd)
	if err != nil {
		return "", err
	}
	return out, nil
}

func (s *service) InitNodeMonitor() (string, error) {
	nodes, err := s.repo.GetAllUseServerNode()
	if err != nil {
		return "", err
	}
	err = utils.NodeMonitor(nodes)
	if err != nil {
		return "", err
	}
	return "success", nil
}
