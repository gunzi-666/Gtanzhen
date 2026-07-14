<script setup>
import { onMounted, ref } from 'vue'
import { api } from '../../api'

// ==== 安装设置 + 到期提醒（同一份 settings） ====
const settings = ref({
  github_repo: '', public_ws_url: '', agent_name: '',
  expire_notify_enabled: false, expire_notify_time: '09:00',
})
const savingSettings = ref(false)

// 自动推导的默认 WS 地址（public_ws_url 留空时生效）。
const autoWS = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/api/agent`

async function saveSettings() {
  savingSettings.value = true
  try {
    await api.put('/api/admin/settings', settings.value)
    alert('设置已保存')
  } catch (e) {
    alert('保存失败：' + e.message)
  } finally {
    savingSettings.value = false
  }
}

function toggleExpireNotify() {
  if (!settings.value.expire_notify_enabled && !security.value.tg_bound) {
    alert('开启到期提醒前请先绑定 Telegram Bot')
    return
  }
  settings.value.expire_notify_enabled = !settings.value.expire_notify_enabled
}

// ==== Telegram 绑定 ====
const security = ref({ tg_bound: false, tg_chat_id: '', tg_token_masked: '', password_changed: false })
const tgForm = ref({ bot_token: '', chat_id: '' })
const tgCode = ref('')
const tgCodeSent = ref(false)
const tgSending = ref(false)

async function loadSecurity() {
  security.value = await api.get('/api/admin/security')
}

async function sendBindCode() {
  if (!tgForm.value.bot_token.trim() || !tgForm.value.chat_id.trim()) {
    alert('请填写 Bot Token 和 Chat ID')
    return
  }
  tgSending.value = true
  try {
    await api.post('/api/admin/tg/bind/code', tgForm.value)
    tgCodeSent.value = true
    alert('验证码已发送到 Telegram，请查收')
  } catch (e) {
    alert(e.message)
  } finally {
    tgSending.value = false
  }
}

async function confirmBind() {
  if (!tgCode.value.trim()) return
  try {
    await api.post('/api/admin/tg/bind', { code: tgCode.value.trim() })
    alert('绑定成功')
    tgCode.value = ''
    tgCodeSent.value = false
    tgForm.value = { bot_token: '', chat_id: '' }
    await loadSecurity()
  } catch (e) {
    alert(e.message)
  }
}

async function unbind() {
  if (!confirm('确定解除 Telegram 绑定？解绑后将无法修改密码。')) return
  await api.post('/api/admin/tg/unbind', {})
  await loadSecurity()
}

// ==== 修改密码 ====
const pwForm = ref({ old_password: '', new_password: '', new_password2: '', code: '' })
const pwCodeSent = ref(false)
const pwSending = ref(false)

async function sendPwCode() {
  pwSending.value = true
  try {
    await api.post('/api/admin/password/code', {})
    pwCodeSent.value = true
    alert('验证码已发送到已绑定的 Telegram')
  } catch (e) {
    alert(e.message)
  } finally {
    pwSending.value = false
  }
}

async function changePassword() {
  const f = pwForm.value
  if (!f.old_password || !f.new_password || !f.code.trim()) {
    alert('请填写完整')
    return
  }
  if (f.new_password.length < 8) {
    alert('新密码至少 8 位')
    return
  }
  if (f.new_password !== f.new_password2) {
    alert('两次输入的新密码不一致')
    return
  }
  try {
    await api.post('/api/admin/password', {
      old_password: f.old_password,
      new_password: f.new_password,
      code: f.code.trim(),
    })
    alert('密码已修改，请重新登录')
    location.hash = '#/login'
  } catch (e) {
    alert(e.message)
  }
}

onMounted(async () => {
  ;[settings.value] = await Promise.all([api.get('/api/admin/settings'), loadSecurity()])
})
</script>

<template>
  <div>
    <div class="page-head">
      <h2>设置</h2>
    </div>

    <div class="card section">
      <h3>Agent 安装设置</h3>
      <p class="muted">用于「服务器」页生成一键安装命令，保存后立即生效。</p>
      <div class="form-row">
        <label>GitHub 仓库（owner/name，用于下载 Agent）</label>
        <input v-model="settings.github_repo" placeholder="例如 gunzi-666/Gtanzhen" />
      </div>
      <div class="form-row">
        <label>面板对外 WS 地址（Agent 连接用，留空 = 自动使用当前访问地址）</label>
        <input v-model="settings.public_ws_url" :placeholder="`留空自动使用 ${autoWS}`" />
      </div>
      <div class="form-row">
        <label>Agent 实例名（可选，多面板共存时区分服务名）</label>
        <input v-model="settings.agent_name" placeholder="留空 = 默认实例 probe-agent；填 hk 则为 probe-agent-hk" />
      </div>
      <div class="actions">
        <button :disabled="savingSettings" @click="saveSettings">保存</button>
      </div>
    </div>

    <div class="card section">
      <h3>Telegram Bot 绑定</h3>
      <p class="muted">绑定后用于接收修改密码等敏感操作的验证码。向 @BotFather 创建 Bot 获取 Token，Chat ID 可通过 @userinfobot 查询。</p>

      <div v-if="security.tg_bound" class="bound-box">
        <span class="badge online">已绑定</span>
        <span class="muted">Bot：{{ security.tg_token_masked }}　Chat ID：{{ security.tg_chat_id }}</span>
        <button class="danger small" @click="unbind">解除绑定</button>
      </div>

      <template v-else>
        <div class="form-row">
          <label>Bot Token</label>
          <input v-model="tgForm.bot_token" placeholder="123456:ABC-DEF..." />
        </div>
        <div class="form-row">
          <label>Chat ID</label>
          <input v-model="tgForm.chat_id" placeholder="例如 123456789" />
        </div>
        <div class="actions">
          <button class="ghost" :disabled="tgSending" @click="sendBindCode">{{ tgCodeSent ? '重新发送验证码' : '发送验证码' }}</button>
        </div>
        <div v-if="tgCodeSent" class="form-row inline-verify">
          <input v-model="tgCode" placeholder="输入收到的 6 位验证码" maxlength="6" />
          <button @click="confirmBind">确认绑定</button>
        </div>
      </template>
    </div>

    <div class="card section">
      <h3>服务器到期提醒</h3>
      <p class="muted">给服务器设置了「到期时间」后（在服务器编辑里设置），到期前 3 天起每天在指定时间通过 Telegram 发送一条提醒。</p>
      <div class="toggle-row">
        <label class="switch">
          <input type="checkbox" :checked="settings.expire_notify_enabled" @click.prevent="toggleExpireNotify" />
          <span class="slider"></span>
        </label>
        <span>{{ settings.expire_notify_enabled ? '已开启' : '已关闭' }}</span>
        <span v-if="!security.tg_bound" class="warn-tip" style="margin:0">（需先绑定 Telegram Bot 才能开启）</span>
      </div>
      <div class="form-row" style="margin-top:12px">
        <label>每日提醒时间</label>
        <input v-model="settings.expire_notify_time" type="time" style="max-width:160px" />
      </div>
      <div class="actions">
        <button :disabled="savingSettings" @click="saveSettings">保存</button>
      </div>
    </div>

    <div class="card section">
      <h3>修改管理员密码</h3>
      <p v-if="!security.tg_bound" class="warn-tip">出于安全考虑，修改密码前必须先绑定 Telegram Bot 接收验证码。</p>
      <template v-else>
        <p class="muted">
          验证流程：填写旧密码和新密码 → 发送验证码到 Telegram → 输入验证码提交。
          <span v-if="security.password_changed">（当前使用的是后台修改过的密码）</span>
          <span v-else>（当前仍在使用启动参数里的初始密码）</span>
        </p>
        <div class="form-row">
          <label>旧密码</label>
          <input v-model="pwForm.old_password" type="password" autocomplete="current-password" />
        </div>
        <div class="form-row">
          <label>新密码（至少 8 位）</label>
          <input v-model="pwForm.new_password" type="password" autocomplete="new-password" />
        </div>
        <div class="form-row">
          <label>确认新密码</label>
          <input v-model="pwForm.new_password2" type="password" autocomplete="new-password" />
        </div>
        <div class="actions">
          <button class="ghost" :disabled="pwSending" @click="sendPwCode">{{ pwCodeSent ? '重新发送验证码' : '发送验证码到 Telegram' }}</button>
        </div>
        <div v-if="pwCodeSent" class="form-row inline-verify">
          <input v-model="pwForm.code" placeholder="输入收到的 6 位验证码" maxlength="6" />
          <button @click="changePassword">确认修改密码</button>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.section {
  margin-bottom: 16px;
  max-width: 640px;
}
.section h3 {
  margin-bottom: 4px;
}
.section > .muted {
  font-size: 13px;
  margin-bottom: 12px;
}
.bound-box {
  display: flex;
  align-items: center;
  gap: 12px;
}
.inline-verify {
  display: flex;
  gap: 10px;
  margin-top: 10px;
}
.inline-verify input {
  flex: 1;
}
.warn-tip {
  color: var(--yellow);
  font-size: 13px;
}
.toggle-row {
  display: flex;
  align-items: center;
  gap: 10px;
}
.actions {
  margin-top: 8px;
}
</style>
