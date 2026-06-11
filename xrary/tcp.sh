#!/bin/bash
# VPN 节点专用调优 - 长连接 + 防 bufferbloat
# 适用: Debian/Ubuntu/CentOS, 内核 5.x+
set -euo pipefail

[ "$(id -u)" -ne 0 ] && { echo "请用 root 运行"; exit 1; }

# ====== 内核版本检测 ======
KERNEL_MAJOR=$(uname -r | cut -d. -f1)

# ====== 备份 ======
#BAK="/etc/sysctl.conf.bak.$(date +%Y%m%d_%H%M%S)"
#cp /etc/sysctl.conf "$BAK"
#echo "[信息] 已备份: $BAK"

# ====== 适配内存 ======
TOTAL_MEM_MB=$(awk '/MemTotal/{printf "%d", $2/1024}' /proc/meminfo)

if   [ "$TOTAL_MEM_MB" -lt 1024 ]; then
    rmem=67108864                    # 64MB
    tcp_rmem="4096 262144 67108864"
    tcp_wmem="4096 131072 67108864"
elif [ "$TOTAL_MEM_MB" -lt 4096 ]; then
    rmem=134217728                   # 128MB
    tcp_rmem="4096 262144 134217728"
    tcp_wmem="4096 131072 134217728"
else
    rmem=268435456                   # 256MB
    tcp_rmem="4096 262144 268435456"
    tcp_wmem="4096 131072 268435456"
fi

echo "[信息] 内存 ${TOTAL_MEM_MB}MB → 单连接最大缓冲区 $((rmem / 1048576))MB"

# ====== 写入核心配置 ======
cat > /etc/sysctl.conf << EOF
# ====================================================
# VPN 节点调优 — stock BBR + cake qdisc
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S')
# 内存: ${TOTAL_MEM_MB}MB  内核: $(uname -r)
# ====================================================

# --- 文件描述符 ---
fs.file-max = 2000000
fs.inotify.max_user_instances = 65536

# --- 内存与交换（VPN 避免 swap）---
vm.swappiness = 1
vm.vfs_cache_pressure = 200
vm.dirty_ratio = 10
vm.dirty_background_ratio = 5

# --- SYN 队列 + 连接管理 ---
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.core.netdev_max_backlog = 500000
net.ipv4.tcp_max_tw_buckets = 500000
net.ipv4.tcp_fin_timeout = 5
net.ipv4.tcp_tw_reuse = 1

# --- 端口范围 ---
net.ipv4.ip_local_port_range = 1024 65535

# --- TCP 缓冲区（根据内存自动适配）---
net.core.rmem_max = ${rmem}
net.core.wmem_max = ${rmem}
net.ipv4.tcp_rmem = ${tcp_rmem}
net.ipv4.tcp_wmem = ${tcp_wmem}
net.ipv4.tcp_mem = 524288 1048576 4194304
net.ipv4.udp_rmem_min = 8192
net.ipv4.udp_wmem_min = 8192
net.ipv4.tcp_adv_win_scale = -2

# --- 重传（VPN 快失败，不等太久）---
net.ipv4.tcp_retries1 = 2
net.ipv4.tcp_retries2 = 5
net.ipv4.tcp_syn_retries = 2
net.ipv4.tcp_synack_retries = 2
net.ipv4.tcp_orphan_retries = 2

# --- Keepalive（60s 快速发现死连接）---
net.ipv4.tcp_keepalive_time = 60
net.ipv4.tcp_keepalive_intvl = 10
net.ipv4.tcp_keepalive_probes = 3

# --- TCP 优化 ---
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

# --- ECN / MTU（关掉更稳）---
net.ipv4.tcp_ecn = 0
net.ipv4.tcp_mtu_probing = 0

# ====================================================
# 🔑 核心：BBR + cake
# ====================================================
net.core.default_qdisc = cake
net.ipv4.tcp_congestion_control = bbr

# --- 转发 ---
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
EOF

# ====== 写入 conntrack 配置（独立文件，以防模块不可用）======
if modprobe nf_conntrack 2>/dev/null; then
    cat > /etc/sysctl.d/99-conntrack.conf << EOF
net.netfilter.nf_conntrack_max = 524288
net.netfilter.nf_conntrack_tcp_timeout_established = 7200
net.netfilter.nf_conntrack_tcp_timeout_time_wait = 10
EOF
    echo "[信息] conntrack 配置已写入 /etc/sysctl.d/99-conntrack.conf"
else
    echo "[信息] conntrack 模块不可用，跳过（不影响 VPN）"
fi

# ====== 加载 BBR / cake ======
modprobe tcp_bbr 2>/dev/null || true
modprobe sch_cake 2>/dev/null || true

# ====== cake 不可用则回退 fq ======
if ! grep -q CONFIG_NET_SCH_CAKE= /boot/config-$(uname -r) 2>/dev/null && \
   ! zgrep -q NET_SCH_CAKE=y /proc/config.gz 2>/dev/null && \
   ! lsmod 2>/dev/null | grep -q sch_cake; then
    sed -i 's/^net.core.default_qdisc = cake/net.core.default_qdisc = fq/' /etc/sysctl.conf
    echo "[!] cake 不可用，已回退到 fq"
fi

# ====== 应用 ======
if ! sysctl -p; then
    echo "[警告] 部分 sysctl 参数应用失败，请检查上方输出"
fi
[ -f /etc/sysctl.d/99-conntrack.conf ] && sysctl -p /etc/sysctl.d/99-conntrack.conf 2>/dev/null || true

# ====== 文件描述符上限 ======
cat > /etc/security/limits.d/99-vpn-nofile.conf << 'EOF'
* soft nofile 1048576
* hard nofile 1048576
root soft nofile 1048576
root hard nofile 1048576
EOF

# ====== 系统文件描述符上限 ======
sed -i '/^DefaultLimitNOFILE=/d' /etc/systemd/system.conf 2>/dev/null || true
sed -i '/^DefaultLimitNOFILE=/d' /etc/systemd/user.conf   2>/dev/null || true
echo "DefaultLimitNOFILE=1048576" >> /etc/systemd/system.conf
echo "DefaultLimitNOFILE=1048576" >> /etc/systemd/user.conf
systemctl daemon-reexec 2>/dev/null || true

# ====== 验证 ======
echo ""
echo "========== 配置结果 =========="

CONG=$(sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null)
QDISC=$(sysctl -n net.core.default_qdisc 2>/dev/null)

echo "拥塞控制: ${CONG:-未知}"
echo "可用算法: $(sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null)"

# cake 判断：内核编译 / modprobe 加载 / 已用 lsmod
CAKE_STATUS="❌ 不可用"
if grep -q CONFIG_NET_SCH_CAKE=y /boot/config-$(uname -r) 2>/dev/null; then
    CAKE_STATUS="✅ 编译进内核"
elif zgrep -q CONFIG_NET_SCH_CAKE=y /proc/config.gz 2>/dev/null; then
    CAKE_STATUS="✅ 编译进内核"
elif lsmod 2>/dev/null | grep -q sch_cake; then
    CAKE_STATUS="✅ 已加载（模块）"
elif [ "${QDISC}" = "cake" ]; then
    CAKE_STATUS="✅ 运行中（sysctl 确认）"
fi

BBR_STATUS="❌ 不可用"
if grep -q CONFIG_TCP_CONG_BBR=y /boot/config-$(uname -r) 2>/dev/null; then
    BBR_STATUS="✅ 编译进内核"
elif zgrep -q CONFIG_TCP_CONG_BBR=y /proc/config.gz 2>/dev/null; then
    BBR_STATUS="✅ 编译进内核"
elif lsmod 2>/dev/null | grep -q tcp_bbr; then
    BBR_STATUS="✅ 已加载（模块）"
elif echo " $(sysctl -n net.ipv4.tcp_available_congestion_control 2>/dev/null) " | grep -qw bbr; then
    BBR_STATUS="✅ 可用（系统支持）"
fi

echo "BBR 状态:  ${BBR_STATUS}"
echo "cake 状态: ${CAKE_STATUS}"
echo "队列算法: ${QDISC:-未知}"
echo "最大缓冲区: $((rmem / 1048576)) MB"

echo ""
echo "========== 连接样本 =========="
ss -ti state established 2>/dev/null | grep -m 3 'bbr' || echo "(暂无 BBR 连接，需有活跃 TCP 流量才能看到)"

echo ""
#echo "🎉 调优完成！备份: $BAK"
echo "🎉 调优完成！"
