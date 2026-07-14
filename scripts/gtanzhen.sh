#!/usr/bin/env bash
# ==========================================================================
# Gtanzhen 探针管理脚本
#
# 首次运行会把自己安装为 gtanzhen 命令，之后在任意位置输入：
#     gtanzhen
# 即可唤出管理菜单：安装/升级/配置/查看日志/查看版本 面板与 Agent。
#
# 一键安装：
#   curl -fsSL https://raw.githubusercontent.com/gunzi-666/Gtanzhen/main/scripts/gtanzhen.sh -o gtanzhen.sh && sudo bash gtanzhen.sh
# ==========================================================================

set -uo pipefail

# ==== 可配置项 ====
REPO="${REPO:-gunzi-666/Gtanzhen}"
VERSION_TAG="${VERSION:-latest}"

SERVER_DIR="/opt/probe"
SERVER_BIN="${SERVER_DIR}/probe-server"
SERVER_SVC="probe-server"

AGENT_DIR="/opt/probe-agent"
AGENT_BIN="${AGENT_DIR}/probe-agent"
AGENT_SVC="probe-agent"

SELF_PATH="/usr/local/bin/gtanzhen"

# ==== 输出辅助 ====
red()    { echo -e "\033[31m$*\033[0m"; }
green()  { echo -e "\033[32m$*\033[0m"; }
yellow() { echo -e "\033[33m$*\033[0m"; }
cyan()   { echo -e "\033[36m$*\033[0m"; }

need_root() {
  if [ "$(id -u)" != "0" ]; then
    red "请用 root 运行：sudo gtanzhen"
    exit 1
  fi
}

# 检测架构，输出 GOARCH。
detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    armv7l|armv6l|arm) echo arm ;;
    *) red "不支持的架构：$(uname -m)"; exit 1 ;;
  esac
}

# 下载文件： download <bin_name> <目标路径>
download() {
  local bin_name="$1" dest="$2" url
  if [ "$VERSION_TAG" = "latest" ]; then
    url="https://github.com/${REPO}/releases/latest/download/${bin_name}"
  else
    url="https://github.com/${REPO}/releases/download/${VERSION_TAG}/${bin_name}"
  fi
  cyan "下载 ${bin_name} ..."
  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$dest" || { red "下载失败：$url"; return 1; }
  else
    wget -O "$dest" "$url" || { red "下载失败：$url"; return 1; }
  fi
  chmod +x "$dest"
}

# 把脚本安装为 gtanzhen 命令。
install_self() {
  if [ "${0}" != "$SELF_PATH" ]; then
    cp -f "$0" "$SELF_PATH" 2>/dev/null && chmod +x "$SELF_PATH" \
      && green "已安装管理命令：以后直接输入 gtanzhen 即可唤出菜单"
  fi
}

svc_active() { systemctl is-active --quiet "$1"; }

# ==== 面板操作 ====
install_server() {
  need_root
  local arch; arch=$(detect_arch)
  read -rp "监听端口 [8008]: " port; port=${port:-8008}
  read -rp "管理员用户名 [admin]: " user; user=${user:-admin}
  read -rp "管理员密码 [随机生成]: " pass
  if [ -z "$pass" ]; then
    pass=$(head -c 12 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 12)
  fi

  mkdir -p "$SERVER_DIR"
  download "probe-server-linux-${arch}" "$SERVER_BIN" || return 1

  cat > "/etc/systemd/system/${SERVER_SVC}.service" <<EOF
[Unit]
Description=Probe Monitoring Server
After=network.target

[Service]
Type=simple
WorkingDirectory=${SERVER_DIR}
Environment=PROBE_ADDR=:${port}
Environment=PROBE_DB=${SERVER_DIR}/probe.db
Environment=PROBE_USER=${user}
Environment=PROBE_PASS=${pass}
ExecStart=${SERVER_BIN}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "$SERVER_SVC" >/dev/null 2>&1
  systemctl restart "$SERVER_SVC"
  sleep 1
  if svc_active "$SERVER_SVC"; then
    local ip; ip=$(hostname -I 2>/dev/null | awk '{print $1}')
    green "面板安装成功！"
    echo "  地址：http://${ip:-服务器IP}:${port}"
    echo "  用户名：${user}"
    echo "  密码：  ${pass}"
  else
    red "启动失败，查看日志： journalctl -u ${SERVER_SVC} -n 50"
  fi
}

upgrade_server() {
  need_root
  if [ ! -f "$SERVER_BIN" ]; then red "面板尚未安装"; return 1; fi
  local arch; arch=$(detect_arch)
  cp -f "$SERVER_BIN" "${SERVER_BIN}.bak"
  if download "probe-server-linux-${arch}" "$SERVER_BIN"; then
    systemctl restart "$SERVER_SVC"
    green "面板已升级并重启。旧版本备份在 ${SERVER_BIN}.bak"
  else
    mv -f "${SERVER_BIN}.bak" "$SERVER_BIN"
    red "升级失败，已回滚到旧版本"
  fi
}

config_server() {
  need_root
  local svc_file="/etc/systemd/system/${SERVER_SVC}.service"
  if [ ! -f "$svc_file" ]; then red "面板尚未安装"; return 1; fi
  local cur_port cur_user
  cur_port=$(grep -oP 'PROBE_ADDR=:\K[0-9]+' "$svc_file" || echo 8008)
  cur_user=$(grep -oP 'PROBE_USER=\K.*' "$svc_file" || echo admin)
  read -rp "监听端口 [${cur_port}]: " port; port=${port:-$cur_port}
  read -rp "管理员用户名 [${cur_user}]: " user; user=${user:-$cur_user}
  read -rp "管理员密码 [留空则不修改]: " pass

  sed -i "s|PROBE_ADDR=:[0-9]*|PROBE_ADDR=:${port}|" "$svc_file"
  sed -i "s|PROBE_USER=.*|PROBE_USER=${user}|" "$svc_file"
  if [ -n "$pass" ]; then
    sed -i "s|PROBE_PASS=.*|PROBE_PASS=${pass}|" "$svc_file"
  fi
  systemctl daemon-reload
  systemctl restart "$SERVER_SVC"
  green "配置已更新并重启面板。"
}

# ==== Agent 操作 ====
install_agent() {
  need_root
  local arch; arch=$(detect_arch)
  read -rp "面板 WS 地址（如 ws://1.2.3.4:8008/api/agent）: " server
  read -rp "本机 secret: " secret
  read -rp "禁用远程执行？(y/N): " dis
  local disable=""
  [ "$dis" = "y" ] || [ "$dis" = "Y" ] && disable="-disable-command"
  if [ -z "$server" ] || [ -z "$secret" ]; then red "地址和 secret 不能为空"; return 1; fi

  mkdir -p "$AGENT_DIR"
  download "probe-agent-linux-${arch}" "$AGENT_BIN" || return 1

  cat > "/etc/systemd/system/${AGENT_SVC}.service" <<EOF
[Unit]
Description=Probe Monitoring Agent
After=network.target

[Service]
Type=simple
ExecStart=${AGENT_BIN} -server "${server}" -secret "${secret}" ${disable}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "$AGENT_SVC" >/dev/null 2>&1
  systemctl restart "$AGENT_SVC"
  sleep 1
  if svc_active "$AGENT_SVC"; then
    green "Agent 安装成功并已上线！"
  else
    red "启动失败，查看日志： journalctl -u ${AGENT_SVC} -n 50"
  fi
}

upgrade_agent() {
  need_root
  if [ ! -f "$AGENT_BIN" ]; then red "Agent 尚未安装"; return 1; fi
  local arch; arch=$(detect_arch)
  cp -f "$AGENT_BIN" "${AGENT_BIN}.bak"
  if download "probe-agent-linux-${arch}" "$AGENT_BIN"; then
    systemctl restart "$AGENT_SVC"
    green "Agent 已升级并重启。旧版本备份在 ${AGENT_BIN}.bak"
  else
    mv -f "${AGENT_BIN}.bak" "$AGENT_BIN"
    red "升级失败，已回滚到旧版本"
  fi
}

# ==== 通用服务控制 ====
svc_menu() {
  local svc="$1" name="$2"
  echo ""
  cyan "== ${name} 服务控制 =="
  echo " 1) 启动   2) 停止   3) 重启   4) 状态   5) 实时日志   0) 返回"
  read -rp "选择: " c
  case "$c" in
    1) systemctl start "$svc"  && green "已启动" ;;
    2) systemctl stop "$svc"   && green "已停止" ;;
    3) systemctl restart "$svc" && green "已重启" ;;
    4) systemctl status "$svc" --no-pager ;;
    5) journalctl -u "$svc" -f ;;
    0) return ;;
    *) red "无效选择" ;;
  esac
}

uninstall() {
  local svc="$1" dir="$2" name="$3"
  need_root
  read -rp "确认卸载 ${name}？(y/N): " c
  [ "$c" = "y" ] || [ "$c" = "Y" ] || return
  systemctl disable --now "$svc" >/dev/null 2>&1
  rm -f "/etc/systemd/system/${svc}.service"
  rm -rf "$dir"
  systemctl daemon-reload
  green "${name} 已卸载"
}

show_version() {
  echo ""
  cyan "== 版本信息 =="
  echo "  管理脚本仓库：${REPO}"
  if [ -x "$SERVER_BIN" ]; then
    echo "  面板版本： $("$SERVER_BIN" -version 2>/dev/null || echo 未知)  [$(svc_active "$SERVER_SVC" && echo 运行中 || echo 已停止)]"
  else
    echo "  面板： 未安装"
  fi
  if [ -x "$AGENT_BIN" ]; then
    echo "  Agent 版本：$("$AGENT_BIN" -version 2>/dev/null || echo 未知)  [$(svc_active "$AGENT_SVC" && echo 运行中 || echo 已停止)]"
  else
    echo "  Agent： 未安装"
  fi
}

menu() {
  while true; do
    echo ""
    green "======== Gtanzhen 探针管理 ========"
    echo "  ---- 面板 ----"
    echo "   1) 安装 / 重装面板"
    echo "   2) 升级面板"
    echo "   3) 修改面板配置（端口/账号/密码）"
    echo "   4) 面板服务控制（启停/日志）"
    echo "   5) 卸载面板"
    echo "  ---- Agent ----"
    echo "   6) 安装 Agent"
    echo "   7) 升级 Agent"
    echo "   8) Agent 服务控制（启停/日志）"
    echo "   9) 卸载 Agent"
    echo "  ---- 其它 ----"
    echo "  10) 显示版本信息"
    echo "   0) 退出"
    echo "===================================="
    read -rp "请输入选项: " opt
    case "$opt" in
      1) install_server ;;
      2) upgrade_server ;;
      3) config_server ;;
      4) svc_menu "$SERVER_SVC" "面板" ;;
      5) uninstall "$SERVER_SVC" "$SERVER_DIR" "面板" ;;
      6) install_agent ;;
      7) upgrade_agent ;;
      8) svc_menu "$AGENT_SVC" "Agent" ;;
      9) uninstall "$AGENT_SVC" "$AGENT_DIR" "Agent" ;;
      10) show_version ;;
      0) exit 0 ;;
      *) red "无效选项" ;;
    esac
  done
}

# ==== 入口 ====
need_root
install_self
menu
