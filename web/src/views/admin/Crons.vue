<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../../api'
import { fmtTime } from '../../format'
import { confirmDialog } from '../../dialog'

const crons = ref([])
const servers = ref([])
const logs = ref([])
const showModal = ref(false)
const editing = ref(null)
const form = ref(defaultForm())

// 立即执行控制台。
const execServer = ref(0)
const execCmd = ref('')
const execOutput = ref('')
const execRunning = ref(false)

function defaultForm() {
  return { name: '', command: '', schedule: '0 * * * *', server_ids: '', enabled: true }
}

async function load() {
  ;[crons.value, servers.value, logs.value] = await Promise.all([
    api.get('/api/admin/crons'),
    api.get('/api/admin/servers'),
    api.get('/api/admin/task-logs?limit=30'),
  ])
  if (execServer.value === 0 && servers.value.length > 0) execServer.value = servers.value[0].id
}

function openCreate() {
  editing.value = null
  form.value = defaultForm()
  showModal.value = true
}
function openEdit(c) {
  editing.value = c
  form.value = { ...c }
  showModal.value = true
}

async function save() {
  if (!form.value.name.trim() || !form.value.command.trim()) return
  if (editing.value) {
    await api.put(`/api/admin/crons/${editing.value.id}`, form.value)
  } else {
    await api.post('/api/admin/crons', form.value)
  }
  showModal.value = false
  await load()
}

async function remove(c) {
  if (!(await confirmDialog(`删除计划任务「${c.name}」？`, { title: '删除计划任务', okText: '删除', danger: true }))) return
  await api.del(`/api/admin/crons/${c.id}`)
  await load()
}

async function runExec() {
  if (!execCmd.value.trim() || !execServer.value) return
  execRunning.value = true
  execOutput.value = '执行中...'
  try {
    const r = await api.post('/api/admin/exec', { server_id: execServer.value, command: execCmd.value, timeout: 60 })
    execOutput.value = (r.output || '') + (r.error ? `\n[错误] ${r.error}` : '')
  } catch (e) {
    execOutput.value = '请求失败：' + e.message
  } finally {
    execRunning.value = false
    load()
  }
}

function serverName(id) {
  const s = servers.value.find((x) => x.id === id)
  return s ? s.name : `#${id}`
}
onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h2>计划任务</h2>
      <button @click="openCreate">+ 添加任务</button>
    </div>

    <div class="card">
      <table>
        <thead><tr><th>名称</th><th>命令</th><th>调度(cron)</th><th>目标</th><th>状态</th><th></th></tr></thead>
        <tbody>
          <tr v-for="c in crons" :key="c.id">
            <td>{{ c.name }}</td>
            <td><code class="cmd">{{ c.command }}</code></td>
            <td class="muted">{{ c.schedule }}</td>
            <td class="muted">{{ c.server_ids || '-' }}</td>
            <td><span class="badge" :class="c.enabled ? 'online' : 'offline'">{{ c.enabled ? '启用' : '停用' }}</span></td>
            <td class="row-actions">
              <button class="ghost small" @click="openEdit(c)">编辑</button>
              <button class="danger small" @click="remove(c)">删除</button>
            </td>
          </tr>
          <tr v-if="crons.length === 0"><td colspan="6" class="muted" style="text-align:center">暂无计划任务</td></tr>
        </tbody>
      </table>
    </div>

    <h3 style="margin-top:28px">立即执行</h3>
    <div class="card exec-console">
      <div class="exec-controls">
        <select v-model.number="execServer" class="exec-server">
          <option v-for="s in servers" :key="s.id" :value="s.id">{{ s.name }}</option>
        </select>
        <input v-model="execCmd" placeholder="输入要在目标服务器执行的命令" @keyup.enter="runExec" />
        <button :disabled="execRunning" @click="runExec">执行</button>
      </div>
      <pre v-if="execOutput" class="exec-output">{{ execOutput }}</pre>
      <div class="muted warn-note">注意：远程执行为高危操作，命令经面板签名后下发；Agent 可用 -disable-command 单方面禁用。</div>
    </div>

    <h3 style="margin-top:28px">执行日志</h3>
    <div class="card">
      <table>
        <thead><tr><th>时间</th><th>服务器</th><th>类型</th><th>结果</th><th>输出</th></tr></thead>
        <tbody>
          <tr v-for="l in logs" :key="l.id">
            <td class="muted">{{ fmtTime(l.created_at) }}</td>
            <td>{{ serverName(l.server_id) }}</td>
            <td>{{ l.cron_id ? '计划' : '手动' }}</td>
            <td><span class="badge" :class="l.success ? 'online' : 'offline'">{{ l.success ? '成功' : '失败' }}</span></td>
            <td><code class="log-out">{{ (l.output || '').slice(0, 200) }}</code></td>
          </tr>
          <tr v-if="logs.length === 0"><td colspan="5" class="muted" style="text-align:center">暂无日志</td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="showModal" class="modal-mask" @click.self="showModal = false">
      <div class="modal">
        <h3>{{ editing ? '编辑' : '添加' }}计划任务</h3>
        <div class="form-row"><label>名称</label><input v-model="form.name" /></div>
        <div class="form-row"><label>命令</label><textarea v-model="form.command" rows="2"></textarea></div>
        <div class="form-row">
          <label>cron 表达式（分 时 日 月 周）</label>
          <input v-model="form.schedule" placeholder="0 * * * * 表示每小时" />
        </div>
        <div class="form-row"><label>目标服务器 ID（逗号分隔）</label><input v-model="form.server_ids" placeholder="例如 1,2" /></div>
        <div class="form-row checkbox-row"><label><input type="checkbox" v-model="form.enabled" class="cb" /> 启用</label></div>
        <div class="actions">
          <button class="ghost" @click="showModal = false">取消</button>
          <button @click="save">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.row-actions { display: flex; gap: 6px; }
.checkbox-row .cb { width: auto; margin-right: 6px; }
.cmd, .log-out { font-family: monospace; font-size: 12px; }
.exec-controls { display: flex; gap: 10px; }
.exec-server { width: 180px; }
.exec-output {
  background: var(--bg-soft);
  padding: 12px;
  border-radius: 8px;
  margin-top: 12px;
  max-height: 300px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
}
.warn-note { margin-top: 10px; font-size: 12px; }
</style>
