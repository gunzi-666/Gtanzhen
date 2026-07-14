#!/usr/bin/env bash
# 探针 Agent 一键安装上线脚本（Linux + systemd），支持多实例。
#
# 用法：
#   sudo bash install-agent.sh <面板WS地址> <secret> [--disable-command] [--name 实例名]
# 示例（默认实例）：
#   sudo bash install-agent.sh ws://1.2.3.4:8008/api/agent 你的secret
# 示例（第二个面板，起名 hk，可与默认实例共存）：
#   sudo bash install-agent.sh ws://5.6.7.8:8008/api/agent 另一个secret --name hk
#
# 也可用环境变量：REPO / VERSION / SERVER / SECRET / NAME

set -euo pipefail

REPO="${REPO:-gunzi-666/Gtanzhen}"     # GitHub 仓库 owner/name
VERSION="${VERSION:-latest}"

red()   { echo -e "\033[31m$*\033[0m"; }
green() { echo -e "\033[32m$*\033[0m"; }

# 参数解析：位置参数（非 -- 开头）作为 server/secret，其次环境变量。
SERVER="${SERVER:-}"
SECRET="${SECRET:-}"
NAME="${NAME:-}"
DISABLE_COMMAND=""
pos=()
skip_next=""
for arg in "$@"; do
  if [ -n "$skip_next" ]; then NAME="$arg"; skip_next=""; continue; fi
  case "$arg" in
    --disable-command) DISABLE_COMMAND="-disable-command" ;;
    --name) skip_next=1 ;;
    --name=*) NAME="${arg#--name=}" ;;
    *) pos+=("$arg") ;;
  esac
done
[ -z "$SERVER" ] && [ "${#pos[@]}" -ge 1 ] && SERVER="${pos[0]}"
[ -z "$SECRET" ] && [ "${#pos[@]}" -ge 2 ] && SECRET="${pos[1]}"

# 实例名决定服务名与安装目录，缺省为单实例 probe-agent。
if [ -n "$NAME" ]; then
  SERVICE_NAME="probe-agent-${NAME}"
  INSTALL_DIR="/opt/probe-agent-${NAME}"
else
  SERVICE_NAME="probe-agent"
  INSTALL_DIR="/opt/probe-agent"
fi

if [ "$(id -u)" != "0" ]; then
  red "请用 root 运行：sudo bash install-agent.sh <面板WS地址> <secret> [--name 实例名]"
  exit 1
fi
if [ -z "$SERVER" ] || [ -z "$SECRET" ]; then
  red "用法：sudo bash install-agent.sh <面板WS地址> <secret> [--disable-command] [--name 实例名]"
  exit 1
fi

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  armv7l|armv6l|arm) GOARCH=arm ;;
  *) red "不支持的架构：$arch"; exit 1 ;;
esac

BIN_NAME="probe-agent-linux-${GOARCH}"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${BIN_NAME}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${BIN_NAME}"
fi

green "==> 创建目录 ${INSTALL_DIR}"
mkdir -p "$INSTALL_DIR"

green "==> 下载 Agent 二进制 ${BIN_NAME}"
# 先下到临时文件再 mv 原子替换：直接覆盖运行中的二进制会报 Text file busy。
TMP_BIN="${INSTALL_DIR}/probe-agent.tmp"
if command -v curl >/dev/null 2>&1; then
  curl -fL "$URL" -o "$TMP_BIN"
else
  wget -O "$TMP_BIN" "$URL"
fi
chmod +x "$TMP_BIN"
mv -f "$TMP_BIN" "${INSTALL_DIR}/probe-agent"

green "==> 写入 systemd 服务 ${SERVICE_NAME}"
cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Probe Monitoring Agent (${NAME:-default})
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/probe-agent -server "${SERVER}" -secret "${SECRET}" ${DISABLE_COMMAND}
Restart=always
RestartSec=5
TimeoutStopSec=10

[Install]
WantedBy=multi-user.target
EOF

green "==> 启动服务"
systemctl daemon-reload
systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
systemctl restart "$SERVICE_NAME"

sleep 1
if systemctl is-active --quiet "$SERVICE_NAME"; then
  green "==> Agent 安装成功并已上线！"
  echo "  查看日志： journalctl -u ${SERVICE_NAME} -f"
  echo "  卸载：     systemctl disable --now ${SERVICE_NAME} && rm -rf ${INSTALL_DIR} /etc/systemd/system/${SERVICE_NAME}.service"
else
  red "Agent 启动失败，请查看： journalctl -u ${SERVICE_NAME} -n 50"
  exit 1
fi
