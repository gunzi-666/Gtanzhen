<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../../api'
import { fmtTime, tagColor } from '../../format'
import { copyWithToast, toast } from '../../clipboard'

const servers = ref([])
const showModal = ref(false)
const editing = ref(null)
const form = ref({ name: '', note: '', sort_order: 0, hidden: false, expire_date: '', tags_text: '', group: '' })

function tagStyle(t) {
  const c = tagColor(t)
  return { color: c, borderColor: c + '55', background: c + '1f' }
}

// 标签输入框（逗号分隔，兼容中文逗号）↔ 数组。
function parseTags(text) {
  return text.split(/[,，]/).map((t) => t.trim()).filter(Boolean)
}

const settings = ref({ github_repo: '', public_ws_url: '', agent_name: '' })

async function load() {
  ;[servers.value, settings.value] = await Promise.all([
    api.get('/api/admin/servers'),
    api.get('/api/admin/settings'),
  ])
}

// 面板对外 WS 地址：优先用设置，否则用浏览器当前访问的地址自动推导。
function wsURL() {
  if (settings.value.public_ws_url) return settings.value.public_ws_url
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${location.host}/api/agent`
}

// 生成某台服务器的一键安装命令；设置了实例名则追加 --name。
function installCmd(s) {
  const repo = settings.value.github_repo || 'gunzi-666/Gtanzhen'
  const script = `https://raw.githubusercontent.com/${repo}/main/scripts/install-agent.sh`
  const name = settings.value.agent_name ? ` --name ${settings.value.agent_name}` : ''
  return `curl -fsSL ${script} -o agent.sh && sudo REPO=${repo} bash agent.sh ${wsURL()} ${s.secret}${name}`
}

async function copyCmd(s) {
  await copyWithToast(installCmd(s), '一键安装命令已复制')
}

// 到期时间：unix 秒 ↔ 日期输入框（当天 23:59:59 到期）。
function tsToDate(ts) {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}
function dateToTs(str) {
  if (!str) return 0
  return Math.floor(new Date(`${str}T23:59:59`).getTime() / 1000)
}
// 剩余天数徽章：<0 已到期，<=3 天临期高亮。
function expiryInfo(s) {
  if (!s.expires_at) return null
  const days = Math.ceil((s.expires_at * 1000 - Date.now()) / 86400000)
  if (days < 0) return { text: '已到期', cls: 'crit' }
  if (days <= 3) return { text: `剩 ${days} 天`, cls: 'crit' }
  if (days <= 14) return { text: `剩 ${days} 天`, cls: 'warn' }
  return { text: `剩 ${days} 天`, cls: '' }
}

function openCreate() {
  editing.value = null
  form.value = { name: '', note: '', sort_order: 0, hidden: false, expire_date: '', tags_text: '', group: '' }
  showModal.value = true
}
function openEdit(s) {
  editing.value = s
  form.value = {
    name: s.name, note: s.note, sort_order: s.sort_order, hidden: s.hidden,
    expire_date: tsToDate(s.expires_at),
    tags_text: (s.tags || []).join(', '),
    group: s.group || '',
  }
  showModal.value = true
}

async function save() {
  if (!form.value.name.trim()) return
  if (editing.value) {
    await api.put(`/api/admin/servers/${editing.value.id}`, {
      name: form.value.name,
      note: form.value.note,
      sort_order: form.value.sort_order,
      hidden: form.value.hidden,
      expires_at: dateToTs(form.value.expire_date),
      tags: parseTags(form.value.tags_text),
      group: form.value.group.trim(),
    })
  } else {
    await api.post('/api/admin/servers', { name: form.value.name, note: form.value.note })
  }
  showModal.value = false
  await load()
}

async function remove(s) {
  if (!confirm(`确定删除「${s.name}」及其历史数据？`)) return
  await api.del(`/api/admin/servers/${s.id}`)
  await load()
}

async function copySecret(s) {
  await copyWithToast(s.secret, 'secret 已复制')
}

// Agent 升级：下发后 Agent 下载最新版替换自身并重启，期间会短暂离线。
const upgradingId = ref(0)
async function upgradeAgent(s) {
  if (!confirm(`向「${s.name}」下发 Agent 升级任务？\nAgent 会下载最新版并自动重启，期间短暂离线。`)) return
  upgradingId.value = s.id
  try {
    const res = await api.post(`/api/admin/servers/${s.id}/upgrade`, {})
    toast(res.output || '升级任务已完成，Agent 正在重启')
    // 等 Agent 重启回连后刷新版本号。
    setTimeout(load, 8000)
  } catch (e) {
    toast('升级失败：' + (e.message || e), 3500)
  } finally {
    upgradingId.value = 0
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h2>服务器</h2>
      <div class="head-btns">
        <RouterLink class="btn-like ghost" to="/admin/settings">安装设置</RouterLink>
        <button @click="openCreate">+ 添加服务器</button>
      </div>
    </div>

    <div class="card table-card">
      <table>
        <thead>
          <tr>
            <th>ID</th><th>名称</th><th>状态</th><th>版本</th><th>到期</th><th>备注</th><th>Secret</th><th>创建时间</th><th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in servers" :key="s.id">
            <td class="muted">{{ s.id }}</td>
            <td>
              {{ s.name }}
              <span v-if="s.group" class="chip">{{ s.group }}</span>
              <span v-for="t in (s.tags || [])" :key="t" class="tag" :style="tagStyle(t)">{{ t }}</span>
              <span v-if="s.hidden" class="chip">隐藏</span>
            </td>
            <td>
              <span class="badge" :class="s.online ? 'online' : 'offline'">{{ s.online ? '在线' : '离线' }}</span>
            </td>
            <td class="muted">{{ s.agent_version || '-' }}</td>
            <td>
              <template v-if="expiryInfo(s)">
                <span class="expiry" :class="expiryInfo(s).cls">{{ tsToDate(s.expires_at) }}（{{ expiryInfo(s).text }}）</span>
              </template>
              <span v-else class="muted">-</span>
            </td>
            <td class="muted">{{ s.note || '-' }}</td>
            <td>
              <code class="secret">{{ s.secret.slice(0, 8) }}…</code>
              <button class="ghost small" @click="copySecret(s)">复制</button>
            </td>
            <td class="muted">{{ fmtTime(s.created_at) }}</td>
            <td class="row-actions">
              <button class="ghost small" @click="copyCmd(s)">一键命令</button>
              <button class="ghost small" :disabled="!s.online || upgradingId === s.id" @click="upgradeAgent(s)">
                {{ upgradingId === s.id ? '升级中…' : '升级' }}
              </button>
              <button class="ghost small" @click="openEdit(s)">编辑</button>
              <button class="danger small" @click="remove(s)">删除</button>
            </td>
          </tr>
          <tr v-if="servers.length === 0"><td colspan="9" class="muted" style="text-align:center">暂无服务器</td></tr>
        </tbody>
      </table>
    </div>

    <div class="install-hint card">
      <b>Agent 安装方式：</b>
      <div class="muted" style="margin:6px 0">推荐：点击每台服务器的「一键命令」，复制后在目标机器（Linux）以 root 运行，自动下载 Agent 并注册为 systemd 服务上线。面板地址默认取当前浏览器访问的地址（{{ wsURL() }}），也可在「设置」页固定指定。</div>
      <div class="muted">手动：<code>probe-agent -server {{ wsURL() }} -secret 该服务器的secret</code></div>
      <div class="muted" style="margin-top:6px">多实例：目标机器需要连接多个面板时，在「设置」页填写 Agent 实例名，一键命令会自动追加 <code style="display:inline;padding:2px 6px">--name 实例名</code>，多个 Agent 可共存互不影响。当前实例名：<b>{{ settings.agent_name || '（默认 probe-agent）' }}</b></div>
      <div v-if="!settings.github_repo" class="warn-tip">
        提示：尚未在「设置」页填写 GitHub 仓库，一键命令暂用默认仓库。
      </div>
    </div>

    <div v-if="showModal" class="modal-mask" @click.self="showModal = false">
      <div class="modal">
        <h3>{{ editing ? '编辑服务器' : '添加服务器' }}</h3>
        <div class="form-row">
          <label>名称</label>
          <input v-model="form.name" placeholder="例如 香港节点" />
        </div>
        <div class="form-row">
          <label>备注</label>
          <input v-model="form.note" />
        </div>
        <template v-if="editing">
          <div class="form-row">
            <label>分组（相同分组的服务器在状态页归为一节，留空 = 未分组）</label>
            <input v-model="form.group" placeholder="例如 香港 / 美国 / 生产环境" />
          </div>
          <div class="form-row">
            <label>个性标签（逗号分隔，显示在状态页）</label>
            <input v-model="form.tags_text" placeholder="例如 香港, 大带宽, 生产" />
            <div class="tag-preview" v-if="parseTags(form.tags_text).length">
              <span v-for="t in parseTags(form.tags_text)" :key="t" class="tag" :style="tagStyle(t)">{{ t }}</span>
            </div>
          </div>
          <div class="form-row">
            <label>到期时间（留空 = 不设置，用于到期提醒）</label>
            <input v-model="form.expire_date" type="date" />
          </div>
          <div class="form-row">
            <label>排序值（越小越靠前）</label>
            <input v-model.number="form.sort_order" type="number" />
          </div>
          <div class="form-row checkbox-row">
            <label><input type="checkbox" v-model="form.hidden" class="cb" /> 在公开状态页隐藏</label>
          </div>
        </template>
        <div class="actions">
          <button class="ghost" @click="showModal = false">取消</button>
          <button @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.secret {
  font-family: monospace;
  margin-right: 8px;
}
.row-actions {
  display: flex;
  gap: 6px;
  justify-content: flex-end;
}
.table-card {
  padding: 6px 10px;
}
.install-hint {
  margin-top: 16px;
}
.install-hint code {
  display: block;
  background: var(--muted);
  border: 1px solid var(--border);
  padding: 8px 12px;
  border-radius: 8px;
  margin: 8px 0;
  font-family: monospace;
  font-size: 12.5px;
}
.checkbox-row .cb {
  width: auto;
  margin-right: 6px;
}
.head-btns {
  display: flex;
  gap: 10px;
}
.warn-tip {
  margin-top: 8px;
  color: var(--yellow);
  font-size: 13px;
}
.expiry {
  font-size: 13px;
}
.expiry.warn {
  color: var(--yellow);
}
.expiry.crit {
  color: var(--red);
}
.tag-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 8px;
}
</style>
