#!/usr/bin/env bash
# 探针面板一键部署脚本（Linux + systemd）。
#
# 用法：
#   sudo bash install-server.sh
# 可用环境变量覆盖默认值：
#   REPO        GitHub 仓库，格式 owner/name（默认取脚本内 REPO）
#   VERSION     指定版本 tag（默认 latest）
#   PORT        监听端口（默认 8008）
#   ADMIN_USER  管理员用户名（默认 admin）
#   ADMIN_PASS  管理员密码（默认随机生成）
#
# 示例：
#   sudo PORT=8080 ADMIN_PASS=mypass bash install-server.sh

set -euo pipefail

# ==== 可配置项 ====
REPO="${REPO:-gunzi-666/Gtanzhen}"     # GitHub 仓库 owner/name
VERSION="${VERSION:-latest}"
PORT="${PORT:-8008}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-}"
INSTALL_DIR="/opt/probe"
SERVICE_NAME="probe-server"

red()    { echo -e "\033[31m$*\033[0m"; }
green()  { echo -e "\033[32m$*\033[0m"; }
yellow() { echo -e "\033[33m$*\033[0m"; }

if [ "$(id -u)" != "0" ]; then
  red "请用 root 运行：sudo bash install-server.sh"
  exit 1
fi

# 检测架构。
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) GOARCH=amd64 ;;
  aarch64|arm64) GOARCH=arm64 ;;
  armv7l|armv6l|arm) GOARCH=arm ;;
  *) red "不支持的架构：$arch"; exit 1 ;;
esac

BIN_NAME="probe-server-linux-${GOARCH}"
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${BIN_NAME}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${BIN_NAME}"
fi

if [ -z "$ADMIN_PASS" ]; then
  ADMIN_PASS=$(head -c 12 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 12)
fi

green "==> 创建目录 ${INSTALL_DIR}"
mkdir -p "$INSTALL_DIR"

green "==> 下载面板二进制 ${BIN_NAME}"
if command -v curl >/dev/null 2>&1; then
  curl -fL "$URL" -o "${INSTALL_DIR}/probe-server"
else
  wget -O "${INSTALL_DIR}/probe-server" "$URL"
fi
chmod +x "${INSTALL_DIR}/probe-server"

green "==> 写入 systemd 服务"
cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=Probe Monitoring Server
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
Environment=PROBE_ADDR=:${PORT}
Environment=PROBE_DB=${INSTALL_DIR}/probe.db
Environment=PROBE_USER=${ADMIN_USER}
Environment=PROBE_PASS=${ADMIN_PASS}
ExecStart=${INSTALL_DIR}/probe-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

green "==> 安装 gtanzhen 管理命令"
MGR_URL="https://raw.githubusercontent.com/${REPO}/main/scripts/gtanzhen.sh"
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$MGR_URL" -o /usr/local/bin/gtanzhen 2>/dev/null && chmod +x /usr/local/bin/gtanzhen \
    && green "    已安装，以后输入 gtanzhen 可唤出管理菜单" || yellow "    管理命令下载失败，可稍后手动安装"
fi

green "==> 启动服务"
systemctl daemon-reload
systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
systemctl restart "$SERVICE_NAME"

sleep 1
if systemctl is-active --quiet "$SERVICE_NAME"; then
  IP=$(hostname -I 2>/dev/null | awk '{print $1}')
  green "==> 部署成功！"
  echo "  面板地址：http://${IP:-服务器IP}:${PORT}"
  echo "  管理员用户名：${ADMIN_USER}"
  echo "  管理员密码：  ${ADMIN_PASS}"
  echo ""
  echo "  查看日志： journalctl -u ${SERVICE_NAME} -f"
  echo "  重启服务： systemctl restart ${SERVICE_NAME}"
else
  red "服务启动失败，请查看： journalctl -u ${SERVICE_NAME} -n 50"
  exit 1
fi
