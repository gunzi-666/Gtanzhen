<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../../api'

const monitors = ref([])
const servers = ref([])
const showModal = ref(false)
const editing = ref(null)
const form = ref(defaultForm())

function defaultForm() {
  return { name: '', type: 'http_get', target: '', server_id: 0, interval: 60, enabled: true }
}

async function load() {
  ;[monitors.value, servers.value] = await Promise.all([
    api.get('/api/admin/monitors'),
    api.get('/api/admin/servers'),
  ])
  if (form.value.server_id === 0 && servers.value.length > 0) {
    form.value.server_id = servers.value[0].id
  }
}

function openCreate() {
  editing.value = null
  form.value = defaultForm()
  if (servers.value.length > 0) form.value.server_id = servers.value[0].id
  showModal.value = true
}
function openEdit(m) {
  editing.value = m
  form.value = { ...m }
  showModal.value = true
}

async function save() {
  if (!form.value.name.trim() || !form.value.target.trim()) return
  if (editing.value) {
    await api.put(`/api/admin/monitors/${editing.value.id}`, form.value)
  } else {
    await api.post('/api/admin/monitors', form.value)
  }
  showModal.value = false
  await load()
}

async function remove(m) {
  if (!confirm(`删除监控「${m.name}」？`)) return
  await api.del(`/api/admin/monitors/${m.id}`)
  await load()
}

const typeLabel = { ping: 'Ping', tcping: 'TCP', http_get: 'HTTP' }
function serverName(id) {
  const s = servers.value.find((x) => x.id === id)
  return s ? s.name : `#${id}`
}
onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h2>服务监控</h2>
      <button @click="openCreate">+ 添加监控</button>
    </div>

    <div class="card">
      <table>
        <thead><tr><th>名称</th><th>类型</th><th>目标</th><th>探测节点</th><th>间隔</th><th>状态</th><th></th></tr></thead>
        <tbody>
          <tr v-for="m in monitors" :key="m.id">
            <td>{{ m.name }}</td>
            <td><span class="chip">{{ typeLabel[m.type] || m.type }}</span></td>
            <td class="muted">{{ m.target }}</td>
            <td class="muted">{{ serverName(m.server_id) }}</td>
            <td>{{ m.interval }}s</td>
            <td><span class="badge" :class="m.enabled ? 'online' : 'offline'">{{ m.enabled ? '启用' : '停用' }}</span></td>
            <td class="row-actions">
              <button class="ghost small" @click="openEdit(m)">编辑</button>
              <button class="danger small" @click="remove(m)">删除</button>
            </td>
          </tr>
          <tr v-if="monitors.length === 0"><td colspan="7" class="muted" style="text-align:center">暂无监控项</td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="showModal" class="modal-mask" @click.self="showModal = false">
      <div class="modal">
        <h3>{{ editing ? '编辑' : '添加' }}服务监控</h3>
        <div class="form-row"><label>名称</label><input v-model="form.name" /></div>
        <div class="form-row">
          <label>类型</label>
          <select v-model="form.type">
            <option value="http_get">HTTP(S) 可用性</option>
            <option value="tcping">TCP 端口连通</option>
            <option value="ping">ICMP Ping</option>
          </select>
        </div>
        <div class="form-row">
          <label>目标（HTTP 填 URL；TCP 填 host:port；Ping 填 host）</label>
          <input v-model="form.target" :placeholder="form.type === 'http_get' ? 'https://example.com' : (form.type === 'tcping' ? '1.1.1.1:443' : '1.1.1.1')" />
        </div>
        <div class="form-row">
          <label>探测节点（由哪台服务器发起）</label>
          <select v-model.number="form.server_id">
            <option v-for="s in servers" :key="s.id" :value="s.id">{{ s.name }}</option>
          </select>
        </div>
        <div class="form-row"><label>探测间隔（秒）</label><input v-model.number="form.interval" type="number" /></div>
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
</style>
