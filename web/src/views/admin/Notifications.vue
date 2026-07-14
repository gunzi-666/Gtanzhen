<script setup>
import { onMounted, ref, computed } from 'vue'
import { api } from '../../api'
import { toast } from '../../clipboard'
import { confirmDialog } from '../../dialog'

const list = ref([])
const showModal = ref(false)
const editing = ref(null)
const form = ref(defaultForm())

function defaultForm() {
  return {
    name: '',
    type: 'telegram',
    enabled: true,
    tg: { bot_token: '', chat_id: '' },
    email: { host: '', port: 465, username: '', password: '', from: '', to: '' },
    webhook: { url: '', method: 'POST', content_type: 'application/json', body: '' },
  }
}

async function load() {
  list.value = await api.get('/api/admin/notifications')
}

function openCreate() {
  editing.value = null
  form.value = defaultForm()
  showModal.value = true
}

function openEdit(n) {
  editing.value = n
  const f = defaultForm()
  f.name = n.name
  f.type = n.type
  f.enabled = n.enabled
  try {
    const cfg = JSON.parse(n.config)
    if (n.type === 'telegram') f.tg = cfg
    else if (n.type === 'email') f.email = cfg
    else if (n.type === 'webhook') f.webhook = cfg
  } catch {
    // ignore
  }
  form.value = f
  showModal.value = true
}

const configForType = computed(() => {
  const f = form.value
  if (f.type === 'telegram') return f.tg
  if (f.type === 'email') return f.email
  return f.webhook
})

async function save() {
  if (!form.value.name.trim()) return
  const payload = {
    name: form.value.name,
    type: form.value.type,
    enabled: form.value.enabled,
    config: JSON.stringify(configForType.value),
  }
  if (editing.value) {
    await api.put(`/api/admin/notifications/${editing.value.id}`, payload)
  } else {
    await api.post('/api/admin/notifications', payload)
  }
  showModal.value = false
  await load()
}

async function remove(n) {
  if (!(await confirmDialog(`删除通知渠道「${n.name}」？`, { title: '删除通知渠道', okText: '删除', danger: true }))) return
  await api.del(`/api/admin/notifications/${n.id}`)
  await load()
}

async function test(n) {
  try {
    await api.post(`/api/admin/notifications/${n.id}/test`, {})
    toast('测试消息已发送')
  } catch (e) {
    toast('发送失败：' + e.message, 3000)
  }
}

const typeLabel = { telegram: 'Telegram', email: '邮件', webhook: 'Webhook' }
onMounted(load)
</script>

<template>
  <div>
    <div class="page-head">
      <h2>通知渠道</h2>
      <button @click="openCreate">+ 添加渠道</button>
    </div>

    <div class="card">
      <table>
        <thead><tr><th>名称</th><th>类型</th><th>状态</th><th></th></tr></thead>
        <tbody>
          <tr v-for="n in list" :key="n.id">
            <td>{{ n.name }}</td>
            <td><span class="chip">{{ typeLabel[n.type] || n.type }}</span></td>
            <td><span class="badge" :class="n.enabled ? 'online' : 'offline'">{{ n.enabled ? '启用' : '停用' }}</span></td>
            <td class="row-actions">
              <button class="ghost small" @click="test(n)">测试</button>
              <button class="ghost small" @click="openEdit(n)">编辑</button>
              <button class="danger small" @click="remove(n)">删除</button>
            </td>
          </tr>
          <tr v-if="list.length === 0"><td colspan="4" class="muted" style="text-align:center">暂无通知渠道</td></tr>
        </tbody>
      </table>
    </div>

    <div v-if="showModal" class="modal-mask" @click.self="showModal = false">
      <div class="modal">
        <h3>{{ editing ? '编辑' : '添加' }}通知渠道</h3>
        <div class="form-row">
          <label>名称</label>
          <input v-model="form.name" />
        </div>
        <div class="form-row">
          <label>类型</label>
          <select v-model="form.type">
            <option value="telegram">Telegram</option>
            <option value="email">邮件 SMTP</option>
            <option value="webhook">通用 Webhook</option>
          </select>
        </div>

        <template v-if="form.type === 'telegram'">
          <div class="form-row"><label>Bot Token</label><input v-model="form.tg.bot_token" /></div>
          <div class="form-row"><label>Chat ID</label><input v-model="form.tg.chat_id" /></div>
        </template>

        <template v-else-if="form.type === 'email'">
          <div class="form-row"><label>SMTP 主机</label><input v-model="form.email.host" /></div>
          <div class="form-row"><label>端口</label><input v-model.number="form.email.port" type="number" /></div>
          <div class="form-row"><label>用户名</label><input v-model="form.email.username" /></div>
          <div class="form-row"><label>密码/授权码</label><input v-model="form.email.password" type="password" /></div>
          <div class="form-row"><label>发件人（留空用用户名）</label><input v-model="form.email.from" /></div>
          <div class="form-row"><label>收件人（逗号分隔）</label><input v-model="form.email.to" /></div>
        </template>

        <template v-else>
          <div class="form-row"><label>URL</label><input v-model="form.webhook.url" /></div>
          <div class="form-row"><label>方法</label>
            <select v-model="form.webhook.method"><option>POST</option><option>GET</option><option>PUT</option></select>
          </div>
          <div class="form-row"><label>Content-Type</label><input v-model="form.webhook.content_type" /></div>
          <div class="form-row">
            <label v-pre>请求体模板（可用 {{title}} 与 {{body}} 占位符，留空发默认 JSON）</label>
            <textarea v-model="form.webhook.body" rows="3"></textarea>
          </div>
        </template>

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
</style>
