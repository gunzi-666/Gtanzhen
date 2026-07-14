<script setup>
import { useRouter } from 'vue-router'
import { logout } from '../../api'

const router = useRouter()
async function doLogout() {
  await logout()
  router.push('/login')
}
</script>

<template>
  <div class="admin-layout">
    <aside class="sidebar">
      <h1 class="side-brand"><span class="brand-dot"></span> 探针后台</h1>
      <RouterLink class="nav-item" to="/admin/servers">服务器</RouterLink>
      <RouterLink class="nav-item" to="/admin/monitors">服务监控</RouterLink>
      <RouterLink class="nav-item" to="/admin/alerts">告警规则</RouterLink>
      <RouterLink class="nav-item" to="/admin/notifications">通知渠道</RouterLink>
      <RouterLink class="nav-item" to="/admin/crons">计划任务</RouterLink>
      <RouterLink class="nav-item" to="/admin/settings">设置</RouterLink>
      <div class="side-foot">
        <RouterLink class="nav-item" to="/">← 状态页</RouterLink>
        <a class="nav-item" href="#" @click.prevent="doLogout">退出登录</a>
      </div>
    </aside>
    <main class="admin-main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.side-brand {
  font-size: 15px;
  padding: 6px 14px 18px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.side-foot {
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
}
</style>
