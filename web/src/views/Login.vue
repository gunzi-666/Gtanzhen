<script setup>
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { login } from '../api'

const username = ref('admin')
const password = ref('')
const error = ref('')
const loading = ref(false)
const router = useRouter()
const route = useRoute()

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
      <h3><span class="brand-dot"></span> 探针监控 · 登录</h3>
      <div class="form-row">
        <label>用户名</label>
        <input v-model="username" autocomplete="username" />
      </div>
      <div class="form-row">
        <label>密码</label>
        <input v-model="password" type="password" autocomplete="current-password" />
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
}
.login-box h3 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
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
</style>
