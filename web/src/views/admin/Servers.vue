<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../../api'
import { fmtTime } from '../../format'

const servers = ref([])
const showModal = ref(false)
const editing = ref(null)
const form = ref({ name: '', note: '', sort_order: 0, hidden: false })

const settings = ref({ github_repo: '', public_ws_url: '' })
const showSettings = ref(false)

async function load() {
  ;[servers.value, settings.value] = await Promise.all([
    api.get('/api/admin/servers'),
    api.get('/api/admin/settings'),
  ])
}

async function saveSettings() {
  await api.put('/api/admin/settings', settings.value)
  showSettings.value = false
  await load()
}

// 生成某台服务器的一键安装命令。
function installCmd(s) {
  const repo = settings.value.github_repo || 'gunzi-666/Gtanzhen'
  const ws = settings.value.public_ws_url || 'ws://面板IP:8008/api/agent'
  const script = `https://raw.githubusercontent.com/${repo}/main/scripts/install-agent.sh`
  return `curl -fsSL ${script} -o agent.sh && sudo REPO=${repo} bash agent.sh ${ws} ${s.secret}`
}

async function copyCmd(s) {
  try {
    await navigator.clipboard.writeText(installCmd(s))
    alert('一键安装命令已复制')
  } catch {
    prompt('复制安装命令：', installCmd(s))
  }
}

function openCreate() {
  editing.value = null
  form.value = { name: '', note: '', sort_order: 0, hidden: false }
  showModal.value = true
}
function openEdit(s) {
  editing.value = s
  form.value = { name: s.name, note: s.note, sort_order: s.sort_order, hidden: s.hidden }
  showModal.value = true
}

async function save() {
  if (!form.value.name.trim()) return
  if (editing.value) {
    await api.put(`/api/admin/servers/${editing.value.id}`, form.value)
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
  try {
    await navigator.clipboard.writeText(s.secret)
    alert('secret 已复制到剪贴板')
  } catch {
    prompt('复制 secret：', s.secret)
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h2>服务器</h2>
      <div class="head-btns">
        <button class="ghost" @click="showSettings = true">安装设置</button>
        <button @click="openCreate">+ 添加服务器</button>
      </div>
    </div>

    <div class="card">
      <table>
        <thead>
          <tr>
            <th>ID</th><th>名称</th><th>状态</th><th>备注</th><th>Secret</th><th>创建时间</th><th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in servers" :key="s.id">
            <td>{{ s.id }}</td>
            <td>{{ s.name }} <span v-if="s.hidden" class="chip">隐藏</span></td>
            <td>
              <span class="badge" :class="s.online ? 'online' : 'offline'">{{ s.online ? '在线' : '离线' }}</span>
            </td>
            <td class="muted">{{ s.note || '-' }}</td>
            <td>
              <code class="secret">{{ s.secret.slice(0, 8) }}…</code>
              <button class="ghost small" @click="copySecret(s)">复制</button>
            </td>
            <td class="muted">{{ fmtTime(s.created_at) }}</td>
            <td class="row-actions">
              <button class="ghost small" @click="copyCmd(s)">一键命令</button>
              <button class="ghost small" @click="openEdit(s)">编辑</button>
              <button class="danger small" @click="remove(s)">删除</button>
            </td>
          </tr>
          <tr v-if="servers.length === 0"><td colspan="7" class="muted" style="text-align:center">暂无服务器</td></tr>
        </tbody>
      </table>
    </div>

    <div class="install-hint card">
      <b>Agent 安装方式：</b>
      <div class="muted" style="margin:6px 0">推荐：点击每台服务器的「一键命令」，复制后在目标机器（Linux）以 root 运行，自动下载 Agent 并注册为 systemd 服务上线。</div>
      <div class="muted">手动：<code>probe-agent -server {{ settings.public_ws_url || 'ws://面板IP:8008/api/agent' }} -secret 该服务器的secret</code></div>
      <div v-if="!settings.github_repo || !settings.public_ws_url" class="warn-tip">
        提示：请先在「安装设置」里填写 GitHub 仓库与面板对外地址，一键命令才会正确。
      </div>
    </div>

    <div v-if="showSettings" class="modal-mask" @click.self="showSettings = false">
      <div class="modal">
        <h3>安装设置</h3>
        <div class="form-row">
          <label>GitHub 仓库（owner/name，用于下载 Agent）</label>
          <input v-model="settings.github_repo" placeholder="例如 gunzi-666/Gtanzhen" />
        </div>
        <div class="form-row">
          <label>面板对外 WS 地址（Agent 连接用）</label>
          <input v-model="settings.public_ws_url" placeholder="ws://你的面板IP:8008/api/agent" />
        </div>
        <div class="actions">
          <button class="ghost" @click="showSettings = false">取消</button>
          <button @click="saveSettings">保存</button>
        </div>
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
}
.install-hint {
  margin-top: 16px;
}
.install-hint code {
  display: block;
  background: var(--bg-soft);
  padding: 8px 12px;
  border-radius: 8px;
  margin: 8px 0;
  font-family: monospace;
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
</style>
