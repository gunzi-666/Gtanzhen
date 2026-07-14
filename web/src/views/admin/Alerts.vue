<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../../api'
import { fmtTime } from '../../format'
import { confirmDialog } from '../../dialog'

const rules = ref([])
const notifications = ref([])
const events = ref([])
const showModal = ref(false)
const editing = ref(null)
const form = ref(defaultForm())

function defaultForm() {
  return {
    name: '',
    metric: 'cpu',
    operator: 'gt',
    threshold: 90,
    duration: 300,
    server_ids: '',
    notification_id: 0,
    enabled: true,
  }
}

async function load() {
  ;[rules.value, notifications.value, events.value] = await Promise.all([
    api.get('/api/admin/alerts'),
    api.get('/api/admin/notifications'),
    api.get('/api/admin/alert-events?limit=50'),
  ])
}

function openCreate() {
  editing.value = null
  form.value = defaultForm()
  showModal.value = true
}
function openEdit(r) {
  editing.value = r
  form.value = { ...r }
  showModal.value = true
}

async function save() {
  if (!form.value.name.trim()) return
  if (editing.value) {
    await api.put(`/api/admin/alerts/${editing.value.id}`, form.value)
  } else {
    await api.post('/api/admin/alerts', form.value)
  }
  showModal.value = false
  await load()
}

async function remove(r) {
  if (!(await confirmDialog(`删除规则「${r.name}」？`, { title: '删除告警规则', okText: '删除', danger: true }))) return
  await api.del(`/api/admin/alerts/${r.id}`)
  await load()
}

const metricLabel = {
  cpu: 'CPU 使用率(%)', mem: '内存使用率(%)', disk: '磁盘使用率(%)',
  load1: '1分钟负载', net_in: '入站速率(B/s)', net_out: '出站速率(B/s)',
  offline: '离线', online: '上线',
}
// 无阈值/持续时间概念的事件型规则。
const eventMetrics = ['offline', 'online']
function notifName(id) {
  const n = notifications.value.find((x) => x.id === id)
  return n ? n.name : '无'
}
onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h2>告警规则</h2>
      <button @click="openCreate">+ 添加规则</button>
    </div>

    <div class="card">
      <table>
        <thead><tr><th>名称</th><th>条件</th><th>持续</th><th>范围</th><th>通知</th><th>状态</th><th></th></tr></thead>
        <tbody>
          <tr v-for="r in rules" :key="r.id">
            <td>{{ r.name }}</td>
            <td>
              <span v-if="eventMetrics.includes(r.metric)">服务器{{ metricLabel[r.metric] }}</span>
              <span v-else>{{ metricLabel[r.metric] }} {{ r.operator === 'gt' ? '>' : '<' }} {{ r.threshold }}</span>
            </td>
            <td>{{ eventMetrics.includes(r.metric) ? '-' : r.duration + 's' }}</td>
            <td class="muted">{{ r.server_ids ? r.server_ids : '全部' }}</td>
            <td class="muted">{{ notifName(r.notification_id) }}</td>
            <td><span class="badge" :class="r.enabled ? 'online' : 'offline'">{{ r.enabled ? '启用' : '停用' }}</span></td>
            <td class="row-actions">
              <button class="ghost small" @click="openEdit(r)">编辑</button>
              <button class="danger small" @click="remove(r)">删除</button>
            </td>
          </tr>
          <tr v-if="rules.length === 0"><td colspan="7" class="muted" style="text-align:center">暂无规则</td></tr>
        </tbody>
      </table>
    </div>

    <h3 style="margin-top:28px">最近告警事件</h3>
    <div class="card">
      <table>
        <thead><tr><th>时间</th><th>状态</th><th>消息</th></tr></thead>
        <tbody>
          <tr v-for="e in events" :key="e.id">
            <td class="muted">{{ fmtTime(e.created_at) }}</td>
            <td><span class="badge" :class="e.state === 'resolved' ? 'online' : 'offline'">{{ e.state === 'resolved' ? '恢复' : '触发' }}</span></td>
            <td>{{ e.message }}</td>
          </tr>
          <tr v-if="events.length === 0"><td colspan="3" class="muted" style="text-align:center">暂无事件</td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="showModal" class="modal-mask" @click.self="showModal = false">
      <div class="modal">
        <h3>{{ editing ? '编辑' : '添加' }}告警规则</h3>
        <div class="form-row"><label>名称</label><input v-model="form.name" /></div>
        <div class="form-row">
          <label>监控指标</label>
          <select v-model="form.metric">
            <option value="cpu">CPU 使用率(%)</option>
            <option value="mem">内存使用率(%)</option>
            <option value="disk">磁盘使用率(%)</option>
            <option value="load1">1分钟负载</option>
            <option value="net_in">入站速率(B/s)</option>
            <option value="net_out">出站速率(B/s)</option>
            <option value="offline">服务器离线</option>
            <option value="online">服务器上线</option>
          </select>
        </div>
        <template v-if="!eventMetrics.includes(form.metric)">
          <div class="form-row inline">
            <div>
              <label>比较</label>
              <select v-model="form.operator"><option value="gt">大于</option><option value="lt">小于</option></select>
            </div>
            <div>
              <label>阈值</label>
              <input v-model.number="form.threshold" type="number" step="any" />
            </div>
            <div>
              <label>持续(秒)</label>
              <input v-model.number="form.duration" type="number" />
            </div>
          </div>
        </template>
        <div class="form-row">
          <label>作用服务器 ID（逗号分隔，留空=全部）</label>
          <input v-model="form.server_ids" placeholder="例如 1,2,3" />
        </div>
        <div class="form-row">
          <label>通知渠道</label>
          <select v-model.number="form.notification_id">
            <option :value="0">不通知</option>
            <option v-for="n in notifications" :key="n.id" :value="n.id">{{ n.name }}</option>
          </select>
        </div>
        <div class="form-row checkbox-row">
          <label><input type="checkbox" v-model="form.enabled" class="cb" /> 启用</label>
        </div>
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
.form-row.inline { display: flex; gap: 10px; }
.form-row.inline > div { flex: 1; }
</style>
