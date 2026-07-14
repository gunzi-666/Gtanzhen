<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { logout, fetchSite } from '../../api'

const router = useRouter()
const siteName = ref('探针监控')

onMounted(async () => {
  try {
    const s = await fetchSite()
    if (s.site_name) siteName.value = s.site_name
  } catch {
    // 用默认值
  }
})

async function doLogout() {
  await logout()
  router.push('/login')
}
</script>

<template>
  <div class="admin-layout">
    <aside class="sidebar">
      <h1 class="side-brand"><span class="brand-dot"></span> {{ siteName }}</h1>
      <RouterLink class="nav-item" to="/admin/servers">服务器</RouterLink>
      <RouterLink class="nav-item" to="/admin/monitors">服务监控</RouterLink>
      <RouterLink class="nav-item" to="/admin/alerts">告警规则</RouterLink>
      <RouterLink class="nav-item" to="/admin/notifications">通知渠道</RouterLink>
      <RouterLink class="nav-item" to="/admin/crons">计划任务</RouterLink>
      <RouterLink class="nav-item" to="/admin/settings">设置</RouterLink>
      <div class="side-foot">
        <a class="nav-item" href="#" @click.prevent="doLogout">退出登录</a>
      </div>
    </aside>
    <main class="admin-main">
      <div class="admin-topline">
        <a class="btn-like ghost" href="#/" target="_blank" rel="noopener">访问状态页 ↗</a>
      </div>
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
.admin-topline {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 14px;
}
.admin-topline .btn-like {
  padding: 4px 12px;
  font-size: 12.5px;
  color: var(--text-dim);
}
</style>
