<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { login, fetchSite } from '../api'

const username = ref('admin')
const password = ref('')
const error = ref('')
const loading = ref(false)
const router = useRouter()
const route = useRoute()

const siteName = ref('探针监控')
onMounted(async () => {
  try {
    const s = await fetchSite()
    if (s.site_name) {
      siteName.value = s.site_name
      document.title = s.site_name
    }
  } catch {
    // 用默认值
  }
})

// ==== 小人动画状态 ====
// 输入用户名时眼睛盯着输入框（瞳孔随输入长度移动），输入密码时捂眼睛。
const userFocused = ref(false)
const passFocused = ref(false)

const pupil = computed(() => {
  if (!userFocused.value) return { x: 0, y: 0 }
  const t = Math.min(username.value.length, 24) / 24
  return { x: -5 + t * 10, y: 4.5 }
})

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await login(username.value, password.value)
    router.push(route.query.redirect || '/admin')
  } catch (e) {
    error.value = e.message === 'invalid credentials' ? '用户名或密码错误' : e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-wrap">
    <form class="modal login-box" @submit.prevent="submit">
      <!-- 小人 -->
      <div class="avatar-ring" :class="{ shy: passFocused }">
        <svg class="avatar" viewBox="0 0 200 200" aria-hidden="true">
          <!-- 身体 -->
          <circle cx="100" cy="115" r="78" class="body" />
          <!-- 肚皮 -->
          <ellipse cx="100" cy="150" rx="45" ry="36" class="belly" />
          <!-- 眼睛 -->
          <g class="eyes" :class="{ closed: passFocused }">
            <g class="eye">
              <circle cx="72" cy="92" r="15" class="eye-white" />
              <circle :cx="72 + pupil.x" :cy="92 + pupil.y" r="6.5" class="pupil" />
              <circle :cx="74.5 + pupil.x" :cy="89.5 + pupil.y" r="2" class="glint" />
            </g>
            <g class="eye">
              <circle cx="128" cy="92" r="15" class="eye-white" />
              <circle :cx="128 + pupil.x" :cy="92 + pupil.y" r="6.5" class="pupil" />
              <circle :cx="130.5 + pupil.x" :cy="89.5 + pupil.y" r="2" class="glint" />
            </g>
          </g>
          <!-- 闭眼线（捂脸时露出的眯眯眼） -->
          <g class="closed-eyes" :class="{ show: passFocused }">
            <path d="M60 92 Q72 100 84 92" />
            <path d="M116 92 Q128 100 140 92" />
          </g>
          <!-- 嘴 -->
          <path class="mouth" :d="passFocused ? 'M88 122 Q100 116 112 122' : 'M86 118 Q100 132 114 118'" />
          <!-- 腮红 -->
          <ellipse cx="55" cy="112" rx="9" ry="5" class="blush" />
          <ellipse cx="145" cy="112" rx="9" ry="5" class="blush" />
          <!-- 手：默认垂在两侧，输密码时举起捂眼 -->
          <g class="hand hand-l" :class="{ cover: passFocused }">
            <circle cx="45" cy="185" r="20" class="paw" />
            <path d="M33 176 Q37 170 43 174" class="finger" />
            <path d="M45 172 Q50 167 55 172" class="finger" />
          </g>
          <g class="hand hand-r" :class="{ cover: passFocused }">
            <circle cx="155" cy="185" r="20" class="paw" />
            <path d="M145 172 Q150 167 155 172" class="finger" />
            <path d="M157 174 Q163 170 167 176" class="finger" />
          </g>
        </svg>
      </div>

      <h3>{{ siteName }} · 登录</h3>
      <div class="form-row">
        <label>用户名</label>
        <input
          v-model="username"
          autocomplete="username"
          @focus="userFocused = true"
          @blur="userFocused = false"
        />
      </div>
      <div class="form-row">
        <label>密码</label>
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          @focus="passFocused = true"
          @blur="passFocused = false"
        />
      </div>
      <div v-if="error" class="err">{{ error }}</div>
      <button type="submit" :disabled="loading" style="width: 100%">
        {{ loading ? '登录中...' : '登录' }}
      </button>
      <RouterLink to="/" class="back muted">返回状态页</RouterLink>
    </form>
  </div>
</template>

<style scoped>
.login-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
}
.login-box {
  width: 360px;
  padding-top: 18px;
}
.login-box h3 {
  text-align: center;
  margin: 14px 0 20px;
}
.err {
  color: var(--red);
  margin-bottom: 12px;
  font-size: 13px;
}
.back {
  display: block;
  text-align: center;
  margin-top: 16px;
  font-size: 13px;
}

/* ==== 小人 ==== */
.avatar-ring {
  width: 120px;
  height: 120px;
  margin: 0 auto;
  border-radius: 50%;
  overflow: hidden;
  border: 2px solid var(--border);
  background: linear-gradient(180deg, var(--accent) 0%, #6366f1 100%);
  transition: transform 0.3s;
}
.avatar-ring.shy {
  transform: scale(0.97);
}
.avatar {
  width: 100%;
  height: 100%;
  display: block;
}
.body {
  fill: #fff;
}
.belly {
  fill: #f1f5f9;
}
.eye-white {
  fill: #fff;
  stroke: #0f172a;
  stroke-width: 3;
}
.pupil {
  fill: #0f172a;
  transition: cx 0.15s ease, cy 0.15s ease;
}
.glint {
  fill: #fff;
  transition: cx 0.15s ease, cy 0.15s ease;
}
.eyes {
  transition: opacity 0.15s;
}
.eyes.closed {
  opacity: 0;
}
.closed-eyes path {
  fill: none;
  stroke: #0f172a;
  stroke-width: 3.5;
  stroke-linecap: round;
  opacity: 0;
  transition: opacity 0.2s 0.15s;
}
.closed-eyes.show path {
  opacity: 1;
}
.mouth {
  fill: none;
  stroke: #0f172a;
  stroke-width: 3.5;
  stroke-linecap: round;
  transition: d 0.25s;
}
.blush {
  fill: #fda4af;
  opacity: 0.65;
}
.paw {
  fill: #fff;
  stroke: #0f172a;
  stroke-width: 3;
}
.finger {
  fill: none;
  stroke: #0f172a;
  stroke-width: 2.5;
  stroke-linecap: round;
}
.hand {
  transition: transform 0.35s cubic-bezier(0.68, -0.4, 0.32, 1.4);
}
.hand-l.cover {
  transform: translate(27px, -94px);
}
.hand-r.cover {
  transform: translate(-27px, -94px);
}
</style>
