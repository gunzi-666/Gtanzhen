#!/usr/bin/env bash
# 探针 Agent 一键安装上线脚本（Linux + systemd）。
#
# 用法：
#   sudo bash install-agent.sh <面板WS地址> <secret> [--disable-command]
# 示例：
#   sudo bash install-agent.sh ws://1.2.3.4:8008/api/agent 你的secret
#
# 也可用环境变量：
#   REPO     GitHub 仓库 owner/name
#   VERSION  版本 tag（默认 latest）
#   SERVER   面板 WS 地址
#   SECRET   本机 secret

set -euo pipefail

REPO="${REPO:-gunzi-666/Gtanzhen}"     # GitHub 仓库 owner/name
VERSION="${VERSION:-latest}"
INSTALL_DIR="/opt/probe-agent"
SERVICE_NAME="probe-agent"

red()   { echo -e "\033[31m$*\033[0m"; }
green() { echo -e "\033[32m$*\033[0m"; }

# 参数解析：位置参数优先，其次环境变量。
SERVER="${1:-${SERVER:-}}"
SECRET="${2:-${SECRET:-}}"
DISABLE_COMMAND=""
for arg in "$@"; do
  if [ "$arg" = "--disable-command" ]; then
    DISABLE_COMMAND="-disable-command"
  fi
done

if [ "$(id -u)" != "0" ]; then
  red "请用 root 运行：sudo bash install-agent.sh <面板WS地址> <secret>"
  exit 1
fi
if [ -z "$SERVER" ] || [ -z "$SECRET" ]; then
  red "用法：sudo bash install-agent.sh <面板WS地址> <secret> [--disable-command]"
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
if command -v curl >/dev/null 2>&1; then
  curl -fL "$URL" -o "${INSTALL_DIR}/probe-agent"
else
  wget -O "${INSTALL_DIR}/probe-agent" "$URL"
fi
chmod +x "${INSTALL_DIR}/probe-agent"

green "==> 写入 systemd 服务"
cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Probe Monitoring Agent
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/probe-agent -server "${SERVER}" -secret "${SECRET}" ${DISABLE_COMMAND}
Restart=always
RestartSec=5

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
