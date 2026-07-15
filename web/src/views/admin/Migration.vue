<script setup>
import { copyWithToast } from '../../clipboard'

// 代码块点击复制。
async function copy(text) {
  await copyWithToast(text, '命令已复制')
}

const CMD_BACKUP = `systemctl stop probe-server
tar -czf /root/probe-backup.tar.gz -C /opt/probe probe.db
systemctl start probe-server`

const CMD_SCP = `scp /root/probe-backup.tar.gz root@新机器IP:/root/`

const CMD_INSTALL = `curl -fsSL https://raw.githubusercontent.com/gunzi-666/Gtanzhen/main/scripts/gtanzhen.sh -o gtanzhen.sh && sudo bash gtanzhen.sh`

const CMD_RESTORE = `systemctl stop probe-server
tar -xzf /root/probe-backup.tar.gz -C /opt/probe
systemctl start probe-server`

const CMD_AGENT_SED = `sed -i 's#ws://旧面板地址:端口#ws://新面板地址:端口#' /etc/systemd/system/probe-agent.service
systemctl daemon-reload && systemctl restart probe-agent`
</script>

<template>
  <div>
    <div class="page-head">
      <h2>迁移教程</h2>
    </div>

    <div class="migrate-wrap">
      <!-- 原理说明 -->
      <div class="card intro">
        <div class="intro-icon">💡</div>
        <div>
          <b>迁移原理</b>
          <p class="muted">
            面板的所有数据——服务器登记、secret 密钥、历史指标、告警规则、通知渠道、站点设置——都保存在一个
            SQLite 文件 <code>/opt/probe/probe.db</code> 里。迁移面板本质上就是<b>把这个文件搬到新机器</b>。
            Agent 用 secret 认证身份，与面板 IP 无关，所以迁移后<b>无需重新注册任何服务器</b>，
            只要 Agent 能连上新面板地址即可自动上线。
          </p>
        </div>
      </div>

      <!-- 步骤 -->
      <div class="step card">
        <div class="step-head">
          <span class="step-num">1</span>
          <h3>旧面板：备份数据</h3>
        </div>
        <p class="muted">先停止面板再打包，确保 WAL 缓存落盘不丢最近数据；打包完立即恢复运行，迁移期间旧面板照常服务。</p>
        <pre class="cmd" title="点击复制" @click="copy(CMD_BACKUP)"><code>{{ CMD_BACKUP }}</code></pre>
        <p class="muted">把备份传到新机器（也可以用宝塔、SFTP 等任何方式）：</p>
        <pre class="cmd" title="点击复制" @click="copy(CMD_SCP)"><code>{{ CMD_SCP }}</code></pre>
      </div>

      <div class="step card">
        <div class="step-head">
          <span class="step-num">2</span>
          <h3>新机器：安装面板</h3>
        </div>
        <p class="muted">
          运行一键管理脚本，选 <b>1) 安装 / 重装面板</b>。端口建议与旧面板保持一致，可以少改一处 Agent 配置；
          管理员账号密码这一步随便设，登录用的是 systemd 里的配置，不在数据库内，稍后照常生效。
        </p>
        <pre class="cmd" title="点击复制" @click="copy(CMD_INSTALL)"><code>{{ CMD_INSTALL }}</code></pre>
      </div>

      <div class="step card">
        <div class="step-head">
          <span class="step-num">3</span>
          <h3>新机器：恢复数据</h3>
        </div>
        <p class="muted">停面板 → 解包覆盖数据库 → 重启。完成后登录新面板，所有服务器、历史图表、告警和设置都应该原样出现。</p>
        <pre class="cmd" title="点击复制" @click="copy(CMD_RESTORE)"><code>{{ CMD_RESTORE }}</code></pre>
      </div>

      <div class="step card">
        <div class="step-head">
          <span class="step-num">4</span>
          <h3>让 Agent 指向新面板</h3>
        </div>
        <div class="callout ok">
          <b>情况 A：面板用域名访问</b>
          <p class="muted">只需把域名解析改到新机器 IP，等 DNS 生效后所有 Agent 自动重连，<b>什么都不用改</b>。这也是推荐从一开始就用域名部署的原因。</p>
        </div>
        <div class="callout warn-c">
          <b>情况 B：面板用 IP 访问（IP 变了）</b>
          <p class="muted">每台被控机需要把 Agent 的连接地址改成新面板。任选一种方式：</p>
          <p class="muted">方式一：在被控机上运行 <code>gtanzhen</code>，选 <b>6) 安装 Agent 实例</b> 覆盖重装，填新面板地址和原来的 secret（后台服务器列表可复制）。</p>
          <p class="muted">方式二：直接改 systemd 配置里的地址（默认实例；命名实例把服务名换成 <code>probe-agent-实例名</code>）：</p>
          <pre class="cmd" title="点击复制" @click="copy(CMD_AGENT_SED)"><code>{{ CMD_AGENT_SED }}</code></pre>
        </div>
      </div>

      <div class="step card">
        <div class="step-head">
          <span class="step-num">5</span>
          <h3>验证并下线旧面板</h3>
        </div>
        <ul class="check-list">
          <li>登录新面板后台，确认服务器陆续变为「在线」；</li>
          <li>打开状态页，确认历史图表、月流量数据完整；</li>
          <li>发一条测试通知（通知渠道页），确认 Telegram / 邮件仍可送达；</li>
          <li>一切正常后，在旧机器运行 <code>gtanzhen</code> 选 <b>5) 卸载面板</b> 下线旧面板。</li>
        </ul>
      </div>

      <!-- 注意事项 -->
      <div class="card notes">
        <h3>注意事项</h3>
        <ul>
          <li><b>务必先停面板再拷贝数据库</b>——运行中拷贝可能丢失 WAL 缓存里最近几分钟的数据。</li>
          <li><b>旧面板先别删</b>，确认新面板完全正常后再卸载，迁移期间随时可以回退。</li>
          <li><b>管理员账号密码不在数据库里</b>（存于 systemd 服务配置），新装面板时重新设置即可，不影响其它数据。</li>
          <li>Telegram 绑定、站点设置、状态页密码等都在数据库里，<b>会随迁移自动保留</b>。</li>
          <li>如果旧面板挂了 Nginx 反代 / HTTPS 证书，记得同步迁移对应的配置和证书。</li>
          <li>新旧面板版本建议保持一致（都用脚本装 latest 即可），跨大版本迁移前先看发布说明。</li>
        </ul>
      </div>
    </div>
  </div>
</template>

<style scoped>
.migrate-wrap {
  max-width: 860px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* 原理卡片 */
.intro {
  display: flex;
  gap: 14px;
  padding: 18px 20px;
  border-left: 3px solid var(--accent);
}
.intro-icon {
  font-size: 22px;
  line-height: 1.3;
}
.intro p {
  margin: 6px 0 0;
}

/* 步骤卡片 */
.step {
  padding: 18px 20px;
}
.step-head {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
}
.step-head h3 {
  margin: 0;
  font-size: 15.5px;
}
.step-num {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--primary);
  color: var(--primary-fg);
  font-weight: 700;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.step p {
  margin: 8px 0;
}

/* 命令块 */
.cmd {
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 12px 14px;
  margin: 8px 0;
  cursor: pointer;
  overflow-x: auto;
  transition: border-color 0.15s;
  position: relative;
}
.cmd:hover {
  border-color: var(--card-border-hover);
}
.cmd:hover::after {
  content: '点击复制';
  position: absolute;
  top: 6px;
  right: 10px;
  font-size: 11px;
  color: var(--text-dim);
}
.cmd code {
  font-family: ui-monospace, 'Cascadia Code', Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.7;
  white-space: pre;
  color: var(--text);
}

/* 提示块 */
.callout {
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 12px 14px;
  margin: 10px 0;
}
.callout.ok {
  border-color: rgba(34, 197, 94, 0.35);
  background: rgba(34, 197, 94, 0.06);
}
.callout.warn-c {
  border-color: rgba(234, 179, 8, 0.35);
  background: rgba(234, 179, 8, 0.06);
}
.callout p {
  margin: 6px 0 0;
}

/* 验证清单 */
.check-list {
  margin: 8px 0 0;
  padding-left: 4px;
  list-style: none;
}
.check-list li {
  padding: 5px 0 5px 26px;
  position: relative;
  color: var(--text-dim);
}
.check-list li::before {
  content: '✓';
  position: absolute;
  left: 4px;
  color: var(--green);
  font-weight: 700;
}

/* 注意事项 */
.notes {
  padding: 18px 20px;
  border-left: 3px solid var(--yellow);
}
.notes h3 {
  margin: 0 0 10px;
  font-size: 15px;
}
.notes ul {
  margin: 0;
  padding-left: 18px;
}
.notes li {
  color: var(--text-dim);
  padding: 3px 0;
}

code {
  background: var(--muted);
  border: 1px solid var(--border);
  border-radius: 5px;
  padding: 1px 6px;
  font-size: 12.5px;
  font-family: ui-monospace, 'Cascadia Code', Consolas, monospace;
}
.cmd code {
  background: none;
  border: none;
  padding: 0;
}
</style>
