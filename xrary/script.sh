#!/usr/bin/env bash
set -Eeuo pipefail

XRAY_BASE_URL="https://pub-eed78fedcfb6470ea94589a3771b4e0f.r2.dev/xray"

log() { echo "[INFO] $*"; }
err() { echo "[ERROR] $*" >&2; }

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    err "Please run as root"
    exit 1
  fi
}

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

install_pkg() {
  local pkgs=("$@")

  if has_cmd apt-get; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y "${pkgs[@]}"

  elif has_cmd yum; then
    yum install -y epel-release || true
    yum install -y "${pkgs[@]}"

  elif has_cmd dnf; then
    dnf install -y "${pkgs[@]}"

  elif has_cmd apk; then
    apk add --no-cache "${pkgs[@]}"

  else
    err "Unsupported package manager"
    exit 1
  fi
}

install_pkgs() {
  local need=()

  has_cmd supervisorctl || need+=(supervisor)
  has_cmd wget || need+=(wget)
  has_cmd curl || need+=(curl)
  [[ -f /etc/ssl/certs/ca-certificates.crt ]] || need+=(ca-certificates)

  if [[ "${#need[@]}" -gt 0 ]]; then
    log "Installing packages: ${need[*]}"
    install_pkg "${need[@]}"
  else
    log "Required packages already installed"
  fi
}

setup_supervisor() {
  log "Starting supervisor..."

  if has_cmd systemctl; then
    systemctl enable supervisor 2>/dev/null || systemctl enable supervisord 2>/dev/null || true
    systemctl restart supervisor 2>/dev/null || systemctl restart supervisord 2>/dev/null || true
  elif has_cmd service; then
    service supervisor restart 2>/dev/null || service supervisord restart 2>/dev/null || true
  fi

  if ! has_cmd supervisorctl; then
    err "supervisorctl not found"
    exit 1
  fi
}

download_file() {
  local url="$1"
  local dest="$2"

  log "Downloading: $url -> $dest"
  mkdir -p "$(dirname "$dest")"

  local tmp="${dest}.tmp.$$"

  if has_cmd curl; then
    curl -fsSL --retry 3 --connect-timeout 10 "$url" -o "$tmp"
  elif has_cmd wget; then
    wget -q --tries=3 --timeout=10 -O "$tmp" "$url"
  else
    err "curl/wget not found"
    exit 1
  fi

  mv -f "$tmp" "$dest"
}

download_if_missing() {
  local url="$1"
  local dest="$2"

  if [[ -s "$dest" ]]; then
    log "Exists, skip: $dest"
    return
  fi

  download_file "$url" "$dest"
}

run_remote_script() {
  local name="$1"
  local file="/tmp/${name}"

  download_file "${XRAY_BASE_URL}/${name}" "$file"
  chmod 755 "$file"

  log "Running: $file"
  bash "$file"
}

install_xray() {
  run_remote_script "install.sh"
  run_remote_script "tcp.sh"
}

install_node_exporter() {
  download_if_missing "${XRAY_BASE_URL}/node_exporter" "/opt/node_exporter"
  chmod 755 /opt/node_exporter
}

install_gost() {
  mkdir -p /gost
  download_if_missing "${XRAY_BASE_URL}/gost" "/gost/gost"
  chmod 755 /gost/gost
}

setup_supervisor_conf() {
  download_file "${XRAY_BASE_URL}/supervisor-pub.conf" "/etc/supervisor/conf.d/pub.conf"

  log "Reloading supervisor..."
  supervisorctl reread
  supervisorctl update
}

check_ports() {
  local ports=(24010)

  for p in "${ports[@]}"; do
    if has_cmd ss && ss -lnt | awk '{print $4}' | grep -Eq "(:|\.)${p}$"; then
      err "Port ${p} already in use"
      exit 1
    fi

    if has_cmd netstat && netstat -lnt | awk '{print $4}' | grep -Eq "(:|\.)${p}$"; then
      err "Port ${p} already in use"
      exit 1
    fi
  done
}

main() {
  require_root
  check_ports
  install_pkgs
  setup_supervisor
  install_node_exporter
  install_gost
  setup_supervisor_conf
  install_xray
  log "DONE"
}

main "$@"