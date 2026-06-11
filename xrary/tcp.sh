#!/usr/bin/env bash
# VPN 节点自适应调优 - TCP/UDP + BBR + cake/fq
# 适用: Debian/Ubuntu/CentOS, Linux 5.x+
# 推荐: 4G/8核 VPN 节点
set -euo pipefail

[ "$(id -u)" -ne 0 ] && { echo "[错误] 请用 root 运行"; exit 1; }

SYSCTL_FILE="/etc/sysctl.d/99-vpn-tune.conf"
CONNTRACK_FILE="/etc/sysctl.d/99-vpn-conntrack.conf"
LIMITS_FILE="/etc/security/limits.d/99-vpn-nofile.conf"

echo "========== VPN 节点自适应调优 =========="

# ==============================
# 基础环境检测
# ==============================

KERNEL_VERSION="$(uname -r)"
KERNEL_MAJOR="$(uname -r | cut -d. -f1)"
TOTAL_MEM_MB="$(awk '/MemTotal/{printf "%d", $2/1024}' /proc/meminfo)"
CPU_CORES="$(nproc 2>/dev/null || echo 1)"

DEFAULT_ROUTE="$(ip route get 8.8.8.8 2>/dev/null || true)"
DEV="$(echo "$DEFAULT_ROUTE" | awk '{for(i=1;i<=NF;i++) if($i=="dev") {print $(i+1); exit}}')"

[ -z "${DEV:-}" ] && DEV="未知"

echo "[信息] 内核: ${KERNEL_VERSION}"
echo "[信息] CPU: ${CPU_CORES} 核"
echo "[信息] 内存: ${TOTAL_MEM_MB} MB"
echo "[信息] 出口网卡: ${DEV}"

# ==============================
# 根据内存自适应 buffer
# ==============================
# 4G 机器不要使用 256MB，容易造成内存压力。
# 8G+ 才启用 256MB。

if [ "$TOTAL_MEM_MB" -lt 1024 ]; then
    rmem=33554432                    # 32MB
    tcp_rmem="4096 131072 33554432"
    tcp_wmem="4096 65536 33554432"
    tcp_mem="131072 262144 524288"
    udp_mem="65536 131072 262144"
    conntrack_max=131072
    nofile_limit=262144
elif [ "$TOTAL_MEM_MB" -lt 2048 ]; then
    rmem=67108864                    # 64MB
    tcp_rmem="4096 262144 67108864"
    tcp_wmem="4096 131072 67108864"
    tcp_mem="131072 262144 524288"
    udp_mem="131072 262144 524288"
    conntrack_max=262144
    nofile_limit=524288
elif [ "$TOTAL_MEM_MB" -lt 8192 ]; then
    rmem=134217728                   # 128MB，适合 4G
    tcp_rmem="4096 262144 134217728"
    tcp_wmem="4096 131072 134217728"
    tcp_mem="262144 524288 1048576"
    udp_mem="262144 524288 1048576"
    conntrack_max=262144
    nofile_limit=524288
else
    rmem=268435456                   # 256MB，适合 8G+
    tcp_rmem="4096 262144 268435456"
    tcp_wmem="4096 131072 268435456"
    tcp_mem="524288 1048576 2097152"
    udp_mem="524288 1048576 2097152"
    conntrack_max=524288
    nofile_limit=1048576
fi

echo "[信息] TCP/UDP 最大 socket buffer: $((rmem / 1048576)) MB"
echo "[信息] conntrack_max: ${conntrack_max}"
echo "[信息] nofile limit: ${nofile_limit}"

# ==============================
# 备份旧配置
# ==============================

if [ -f "$SYSCTL_FILE" ]; then
    cp "$SYSCTL_FILE" "${SYSCTL_FILE}.bak.$(date +%Y%m%d_%H%M%S)"
    echo "[信息] 已备份旧配置: ${SYSCTL_FILE}.bak.*"
fi

if [ -f /etc/sysctl.conf ]; then
    cat > /etc/sysctl.conf << EOF
# empty - managed by /etc/sysctl.d/99-vpn-tune.conf
EOF
    echo "[信息] 已清空旧 /etc/sysctl.conf，避免覆盖新配置"
fi


# ==============================
# 加载可选模块
# ==============================

modprobe tcp_bbr 2>/dev/null || true
modprobe sch_cake 2>/dev/null || true
modprobe sch_fq 2>/dev/null || true
modprobe nf_conntrack 2>/dev/null || true

# ==============================
# 检测 BBR 是否可用
# ==============================

bbr_available=0

if sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null | grep -qw bbr; then
    bbr_available=1
fi

if [ "$bbr_available" -eq 1 ]; then
    TCP_CC="bbr"
else
    TCP_CC="cubic"
    echo "[警告] BBR 不可用，自动回退 cubic"
fi

# ==============================
# 检测 qdisc: 优先 cake，其次 fq
# ==============================

qdisc="fq"

if modprobe sch_cake 2>/dev/null || \
   grep -q CONFIG_NET_SCH_CAKE=y "/boot/config-$(uname -r)" 2>/dev/null || \
   zgrep -q CONFIG_NET_SCH_CAKE=y /proc/config.gz 2>/dev/null; then
    qdisc="cake"
else
    qdisc="fq"
fi


echo "[信息] 拥塞控制: ${TCP_CC}"
echo "[信息] 默认队列算法: ${qdisc}"

# ==============================
# 写入 sysctl 主配置
# ==============================

cat > "$SYSCTL_FILE" << EOF
# ====================================================
# VPN 节点自适应调优
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S')
# 内核: ${KERNEL_VERSION}
# CPU: ${CPU_CORES}
# 内存: ${TOTAL_MEM_MB} MB
# 出口网卡: ${DEV}
# ====================================================

# ------------------------------
# 文件描述符
# ------------------------------
fs.file-max = 1000000
fs.inotify.max_user_instances = 65536

# ------------------------------
# 内存与 swap
# ------------------------------
vm.swappiness = 1
vm.vfs_cache_pressure = 200
vm.dirty_ratio = 10
vm.dirty_background_ratio = 5

# ------------------------------
# 网络队列
# ------------------------------
net.core.somaxconn = 65535
net.core.netdev_max_backlog = 250000

# ------------------------------
# TCP 连接管理
# ------------------------------
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.tcp_max_tw_buckets = 500000
net.ipv4.tcp_fin_timeout = 5
net.ipv4.tcp_tw_reuse = 1
net.ipv4.ip_local_port_range = 1024 65535

# ------------------------------
# TCP buffer
# ------------------------------
net.core.rmem_max = ${rmem}
net.core.wmem_max = ${rmem}
net.core.rmem_default = 1048576
net.core.wmem_default = 1048576
net.ipv4.tcp_rmem = ${tcp_rmem}
net.ipv4.tcp_wmem = ${tcp_wmem}
net.ipv4.tcp_mem = ${tcp_mem}
net.ipv4.tcp_adv_win_scale = -2

# ------------------------------
# UDP buffer
# ------------------------------
net.ipv4.udp_rmem_min = 16384
net.ipv4.udp_wmem_min = 16384
net.ipv4.udp_mem = ${udp_mem}

# ------------------------------
# TCP 重传与超时
# ------------------------------
net.ipv4.tcp_retries1 = 2
net.ipv4.tcp_retries2 = 5
net.ipv4.tcp_syn_retries = 2
net.ipv4.tcp_synack_retries = 2
net.ipv4.tcp_orphan_retries = 2

# ------------------------------
# Keepalive
# ------------------------------
net.ipv4.tcp_keepalive_time = 60
net.ipv4.tcp_keepalive_intvl = 10
net.ipv4.tcp_keepalive_probes = 3

# ------------------------------
# TCP 通用优化
# ------------------------------
net.ipv4.tcp_fastopen = 3
net.ipv4.tcp_slow_start_after_idle = 0
net.ipv4.tcp_no_metrics_save = 1
net.ipv4.tcp_moderate_rcvbuf = 1
net.ipv4.tcp_window_scaling = 1
net.ipv4.tcp_sack = 1
net.ipv4.tcp_timestamps = 1
net.ipv4.tcp_syncookies = 1
net.ipv4.tcp_notsent_lowat = 131072
net.ipv4.tcp_autocorking = 0
net.ipv4.tcp_frto = 1

# ------------------------------
# ECN / MTU
# ------------------------------
net.ipv4.tcp_ecn = 0
net.ipv4.tcp_mtu_probing = 0

# ------------------------------
# 拥塞控制 + 队列算法
# ------------------------------
net.core.default_qdisc = ${qdisc}
net.ipv4.tcp_congestion_control = ${TCP_CC}

# ------------------------------
# 转发
# ------------------------------
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
net.ipv6.conf.default.forwarding = 1

# ------------------------------
# ICMP / PMTU
# ------------------------------
net.ipv4.icmp_echo_ignore_broadcasts = 1
net.ipv4.icmp_ignore_bogus_error_responses = 1
EOF

echo "[信息] sysctl 配置已写入: $SYSCTL_FILE"

# ==============================
# conntrack 自适应配置
# ==============================

if [ -d /proc/sys/net/netfilter ]; then
    cat > "$CONNTRACK_FILE" << EOF
# VPN conntrack 调优
net.netfilter.nf_conntrack_max = ${conntrack_max}
net.netfilter.nf_conntrack_tcp_timeout_established = 7200
net.netfilter.nf_conntrack_tcp_timeout_time_wait = 10
net.netfilter.nf_conntrack_tcp_timeout_close_wait = 10
net.netfilter.nf_conntrack_tcp_timeout_fin_wait = 10
net.netfilter.nf_conntrack_udp_timeout = 30
net.netfilter.nf_conntrack_udp_timeout_stream = 180
EOF
    echo "[信息] conntrack 配置已写入: $CONNTRACK_FILE"
else
    echo "[信息] conntrack 不可用，跳过"
fi

# ==============================
# 应用 sysctl
# ==============================

echo "[信息] 正在应用 sysctl..."

if ! sysctl -p "$SYSCTL_FILE"; then
    echo "[警告] 部分 sysctl 参数应用失败，请检查上方输出"
fi

if [ -f "$CONNTRACK_FILE" ]; then
    if ! sysctl -p "$CONNTRACK_FILE"; then
        echo "[警告] conntrack 部分参数应用失败，可忽略"
    fi
fi

# ==============================
# systemd / limits nofile
# ==============================

cat > "$LIMITS_FILE" << EOF
* soft nofile ${nofile_limit}
* hard nofile ${nofile_limit}
root soft nofile ${nofile_limit}
root hard nofile ${nofile_limit}
EOF

sed -i '/^DefaultLimitNOFILE=/d' /etc/systemd/system.conf 2>/dev/null || true
sed -i '/^DefaultLimitNOFILE=/d' /etc/systemd/user.conf 2>/dev/null || true

echo "DefaultLimitNOFILE=${nofile_limit}" >> /etc/systemd/system.conf
echo "DefaultLimitNOFILE=${nofile_limit}" >> /etc/systemd/user.conf

systemctl daemon-reexec 2>/dev/null || true

echo "[信息] 文件描述符限制已设置: ${nofile_limit}"

# ==============================
# 可选：cake 带宽整形
# 用法：
# BANDWIDTH=900mbit ./vpn_tune.sh
# BANDWIDTH=450mbit ./vpn_tune.sh
# ==============================

if [ "${BANDWIDTH:-}" != "" ] && [ "$DEV" != "未知" ]; then
    if command -v tc >/dev/null 2>&1; then
        echo "[信息] 正在设置 cake 带宽整形: dev=${DEV}, bandwidth=${BANDWIDTH}"
        if tc qdisc replace dev "$DEV" root cake bandwidth "$BANDWIDTH" besteffort; then
            echo "[信息] cake 带宽整形已生效"
        else
            echo "[警告] cake 带宽整形设置失败"
        fi
    else
        echo "[警告] 未找到 tc 命令，跳过 cake 带宽整形"
    fi
fi

# ==============================
# Xray systemd 专用 nofile
# ==============================

unit="XrayR.service"

if systemctl cat "$unit" >/dev/null 2>&1; then
    mkdir -p /etc/systemd/system/${unit}.d

    cat >/etc/systemd/system/${unit}.d/override.conf <<EOF
[Service]
LimitNOFILE=${nofile_limit}
LimitNPROC=${nofile_limit}
TasksMax=infinity
EOF

    echo "[信息] 已为 ${unit} 设置 LimitNOFILE=${nofile_limit}"

    systemctl daemon-reexec
    systemctl daemon-reload
    systemctl restart XrayR


    cat > "/etc/security/limits.d/99-nofile.conf" <<EOF
* soft nofile ${nofile_limit}
* hard nofile ${nofile_limit}
root soft nofile ${nofile_limit}
root hard nofile ${nofile_limit}
EOF
    echo "fs.file-max = $((nofile_limit * 2))" >/etc/sysctl.d/99-filemax.conf
    sysctl --system

    echo "[信息] 已为 ${unit} 设置全局 nofile 限制，并重启服务"
else
    echo "[信息] 未找到 ${unit}，跳过"
fi

# ==============================
# 验证输出
# ==============================

echo ""
echo "========== 配置结果 =========="

CONG="$(sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null || echo unknown)"
QDISC="$(sysctl -n net.core.default_qdisc 2>/dev/null || echo unknown)"
AVAILABLE_CC="$(sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null || echo unknown)"

echo "拥塞控制: ${CONG}"
echo "可用算法: ${AVAILABLE_CC}"
echo "队列算法: ${QDISC}"
echo "出口网卡: ${DEV}"
echo "最大 TCP/UDP buffer: $((rmem / 1048576)) MB"
echo "nofile: ${nofile_limit}"

if [ -f "$CONNTRACK_FILE" ]; then
    echo "conntrack_max: $(sysctl -n net.netfilter.nf_conntrack_max 2>/dev/null || echo unknown)"
fi

# BBR 状态
BBR_STATUS="❌ 不可用"
if echo " ${AVAILABLE_CC} " | grep -qw bbr; then
    BBR_STATUS="✅ 可用"
fi
if lsmod 2>/dev/null | grep -q tcp_bbr; then
    BBR_STATUS="✅ 已加载模块"
fi
if grep -q CONFIG_TCP_CONG_BBR=y "/boot/config-$(uname -r)" 2>/dev/null; then
    BBR_STATUS="✅ 编译进内核"
fi

# cake 状态
CAKE_STATUS="❌ 不可用"
if [ "$QDISC" = "cake" ]; then
    CAKE_STATUS="✅ 运行中"
fi
if lsmod 2>/dev/null | grep -q sch_cake; then
    CAKE_STATUS="✅ 已加载模块"
fi
if grep -q CONFIG_NET_SCH_CAKE=y "/boot/config-$(uname -r)" 2>/dev/null; then
    CAKE_STATUS="✅ 编译进内核"
fi

echo "BBR 状态:  ${BBR_STATUS}"
echo "cake 状态: ${CAKE_STATUS}"

if [ "$DEV" != "未知" ] && command -v tc >/dev/null 2>&1; then
    echo ""
    echo "========== qdisc 状态 =========="
    tc -s qdisc show dev "$DEV" | head -30 || true
fi

echo ""
echo "========== BBR 连接样本 =========="
ss -ti state established 2>/dev/null | grep -m 3 'bbr' || echo "(暂无 BBR 连接，需有活跃 TCP 流量才能看到)"

echo ""
echo "🎉 VPN TCP/UDP 自适应调优完成！"
echo "配置文件: $SYSCTL_FILE"
