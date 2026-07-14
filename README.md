# Probe 探针监控

一个自研的轻量级服务器监控系统（面板 + Agent），使用 Go + WebSocket + Vue 3 实现。
设计上参考了哪吒探针的功能形态，但代码为完全重写，并**移除了 SSH / WebShell** 等高风险终端转发功能。

## 功能

- 实时指标监控：CPU、内存、磁盘、网络速率/累计流量、负载、TCP/UDP 连接数、进程数、运行时长。
- 历史图表：分钟级 / 小时级降采样，保留策略自动清理。
- 服务可用性监控：ICMP Ping、TCP 端口、HTTP(S) 探测，由指定 Agent 发起。
- 告警引擎：滑动窗口（持续时长）规则 + 触发/恢复状态机 + 离线告警。
- 通知渠道：Telegram、邮件（SMTP）、通用 Webhook（支持请求体模板）。
- 计划任务与远程执行：cron 定时下发命令、面板即时执行控制台。
- 月流量统计：处理 Agent 重启导致的计数器回退。
- 公开状态页 + 管理后台（单管理员账号 + Session 鉴权）。
- 单二进制部署：前端通过 `go:embed` 内嵌进面板二进制。

## 架构

```
Agent (被控端)  --WebSocket-->  Server (面板)  <--HTTP/WS-->  浏览器
  gopsutil 采集                  Hub 连接池
  任务执行器                     SQLite 存储 + 降采样
                                 告警引擎 / 监控调度 / 计划任务
```

- 通信协议：自定义 JSON 消息信封（见 `internal/protocol`）。
- 存储：SQLite（WAL 模式），`store` 层集中所有 SQL，便于以后替换为 PostgreSQL。

## 安全说明（远程执行）

移除 SSH 后，远程执行命令是唯一的高危通道，已做如下加固：

- 每台机器独立随机 secret，Agent 认证与命令签名均用它。
- `exec_command` 使用 HMAC-SHA256 签名（含 task_id、命令、nonce、时间戳），Agent 侧校验签名并拒绝超过 5 分钟的旧签名（防重放）。
- 命令输出截断至 1MB，默认超时 60 秒。
- Agent 可用 `-disable-command` 单方面禁用远程执行。

## 一键部署（推荐）

> 前提：已在 GitHub 上打过 tag 触发 Release，生成了各平台二进制。

### 部署面板（Linux）

```bash
sudo bash <(curl -fsSL https://raw.githubusercontent.com/gunzi-666/Gtanzhen/main/scripts/install-server.sh)
```

可加环境变量自定义：`PORT`、`ADMIN_USER`、`ADMIN_PASS`、`REPO`、`VERSION`。脚本会下载二进制、注册 systemd 服务并启动，最后打印面板地址与管理员密码，同时安装 `gtanzhen` 管理命令。

### 管理命令 gtanzhen

部署面板后（或单独安装），在服务器上输入 `gtanzhen` 即可唤出交互式管理菜单：

```bash
# 若面板脚本已装则直接：
sudo gtanzhen
# 或单独安装管理命令：
curl -fsSL https://raw.githubusercontent.com/gunzi-666/Gtanzhen/main/scripts/gtanzhen.sh -o gtanzhen.sh && sudo bash gtanzhen.sh
```

菜单功能：安装/升级/卸载面板与 Agent、修改面板配置（端口/账号/密码）、启停与查看实时日志、显示面板与 Agent 版本。Agent 支持多实例（同机连多个面板），升级会先备份旧二进制，失败自动回滚。

### 上线 Agent（Linux）

1. 面板后台「服务器」页先点「安装设置」，填写 GitHub 仓库与面板对外 WS 地址。
2. 每台服务器点「一键命令」，复制后在目标机器以 root 运行即可，例如：

```bash
curl -fsSL https://raw.githubusercontent.com/gunzi-666/Gtanzhen/main/scripts/install-agent.sh -o agent.sh && sudo REPO=gunzi-666/Gtanzhen bash agent.sh ws://面板IP:8008/api/agent 该机secret
```

Agent 会被注册为 `probe-agent` systemd 服务，开机自启、断线自动重连。

**多实例**：同一台机器可以同时连接多个面板。给命令追加 `--name 实例名` 即可再装一份互不冲突的 Agent（服务名 `probe-agent-实例名`，目录 `/opt/probe-agent-实例名`）：

```bash
sudo REPO=owner/repo bash agent.sh ws://另一个面板IP:8008/api/agent 另一个secret --name hk
```

## 自动构建与发布（CI）

仓库包含 [.github/workflows/release.yml](.github/workflows/release.yml)：推送形如 `v1.0.0` 的 tag 时，自动构建前端、交叉编译六个平台（linux amd64/arm64/arm、windows amd64、darwin amd64/arm64）的面板与 Agent 二进制，并发布到 GitHub Release（含 `checksums.txt`）。

```bash
git tag v1.0.0
git push origin v1.0.0
```

## 快速开始（手动构建）

### 1. 构建

需要 Go 1.25+ 与 Node.js 18+。

```bash
# 构建前端
cd web
npm install
npm run build
cd ..

# 构建面板（内嵌前端）
go build -o probe-server ./cmd/server

# 构建 Agent
go build -o probe-agent ./cmd/agent
```

Windows PowerShell 下分别执行上述命令即可（不要使用 `&&` 连接）。

### 2. 启动面板

```bash
probe-server -addr :8008 -db probe.db -user admin -pass 你的密码
```

也可用环境变量 `PROBE_ADDR` / `PROBE_DB` / `PROBE_USER` / `PROBE_PASS`。

打开 `http://面板IP:8008` 查看状态页，`#/login` 进入管理后台。

### 3. 添加服务器并部署 Agent

1. 后台「服务器」页点击「添加服务器」，复制生成的 secret。
2. 在目标机器运行：

```bash
probe-agent -server ws://面板IP:8008/api/agent -secret 复制的secret
```

可选参数：`-period` 上报间隔（秒）、`-disable-command` 禁用远程执行。
也支持环境变量 `PROBE_SERVER` / `PROBE_SECRET`。

## 开发

前端开发（热更新，API 代理到 :8008）：

```bash
cd web
npm run dev
```

## 目录结构

```
cmd/server        面板入口
cmd/agent         Agent 入口
internal/protocol 通信协议（两端共用）
internal/agent    采集、连接、任务执行
internal/server/hub      Agent 连接管理
internal/server/store    SQLite 存储 + 降采样
internal/server/alert    告警引擎 + 通知渠道
internal/server/monitor  服务监控调度
internal/server/task     任务下发与结果回收
internal/server/cron     计划任务调度
internal/server/api      REST + 浏览器 WS 推送 + 内嵌前端
web               Vue 3 前端
```
