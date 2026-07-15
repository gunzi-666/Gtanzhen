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

// 侧边导航：分组 + 图标（内联 SVG path，16px stroke 风格）。
const navGroups = [
  {
    label: '监控',
    items: [
      { to: '/admin/servers', text: '服务器', icon: 'M5 4h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1Zm0 10h14a1 1 0 0 1 1 1v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-4a1 1 0 0 1 1-1Zm2.5-7h.01M7.5 17h.01' },
      { to: '/admin/monitors', text: '服务监控', icon: 'M3 12h4l3-8 4 16 3-8h4' },
    ],
  },
  {
    label: '告警',
    items: [
      { to: '/admin/alerts', text: '告警规则', icon: 'M12 3a6 6 0 0 1 6 6c0 4 1.5 5.5 2 6H4c.5-.5 2-2 2-6a6 6 0 0 1 6-6Zm-2 15a2 2 0 0 0 4 0' },
      { to: '/admin/notifications', text: '通知渠道', icon: 'M21 3 10 14M21 3l-7 18-4-7-7-4 18-7Z' },
    ],
  },
  {
    label: '系统',
    items: [
      { to: '/admin/crons', text: '计划任务', icon: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18Zm0 4v5l3.5 2' },
      { to: '/admin/settings', text: '设置', icon: 'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6Zm7.4 3a7.4 7.4 0 0 0-.1-1.2l2-1.6-2-3.4-2.4 1a7.5 7.5 0 0 0-2-1.2L14.5 3h-5l-.4 2.6a7.5 7.5 0 0 0-2 1.2l-2.4-1-2 3.4 2 1.6a7.4 7.4 0 0 0 0 2.4l-2 1.6 2 3.4 2.4-1a7.5 7.5 0 0 0 2 1.2l.4 2.6h5l.4-2.6a7.5 7.5 0 0 0 2-1.2l2.4 1 2-3.4-2-1.6c.07-.4.1-.8.1-1.2Z' },
      { to: '/admin/migration', text: '迁移教程', icon: 'M12 6.5c-2-1.8-4.7-2-7-1v13c2.3-1 5-0.8 7 1 2-1.8 4.7-2 7-1v-13c-2.3-1-5-0.8-7 1Zm0 0v13' },
    ],
  },
]
</script>

<template>
  <div class="admin-layout">
    <aside class="sidebar">
      <h1 class="side-brand"><span class="brand-dot"></span> {{ siteName }}</h1>

      <nav class="side-nav">
        <div v-for="g in navGroups" :key="g.label" class="nav-group">
          <div class="nav-group-label">{{ g.label }}</div>
          <RouterLink v-for="item in g.items" :key="item.to" class="nav-item" :to="item.to">
            <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
                 stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
              <path :d="item.icon" />
            </svg>
            <span>{{ item.text }}</span>
          </RouterLink>
        </div>
      </nav>

      <div class="side-foot">
        <a class="nav-item" href="#/" target="_blank" rel="noopener">
          <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18Zm-9 9h18M12 3c2.5 2.5 3.5 5.5 3.5 9s-1 6.5-3.5 9c-2.5-2.5-3.5-5.5-3.5-9s1-6.5 3.5-9Z" />
          </svg>
          <span>访问状态页</span>
          <span class="ext-mark">↗</span>
        </a>
        <a class="nav-item logout" href="#" @click.prevent="doLogout">
          <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">
            <path d="M15 4h4a1 1 0 0 1 1 1v14a1 1 0 0 1-1 1h-4M10 17l5-5-5-5M15 12H3" />
          </svg>
          <span>退出登录</span>
        </a>
      </div>
    </aside>
    <main class="admin-main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
}
.side-brand {
  font-size: 15px;
  padding: 8px 12px 16px;
  margin: 0;
  display: flex;
  align-items: center;
  gap: 9px;
  border-bottom: 1px solid var(--border);
  margin-bottom: 12px;
}
.side-nav {
  flex: 1;
}
.nav-group {
  margin-bottom: 14px;
}
.nav-group-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-dim);
  opacity: 0.75;
  padding: 0 12px;
  margin-bottom: 4px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 12px;
  border-radius: var(--radius-sm);
  color: var(--text-dim);
  margin-bottom: 2px;
  font-size: 13.5px;
  font-weight: 500;
  position: relative;
  transition: background 0.12s, color 0.12s;
}
.nav-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  opacity: 0.85;
}
.nav-item:hover {
  background: var(--muted);
  color: var(--text);
}
.nav-item.router-link-active {
  background: var(--card-hover);
  color: var(--text);
}
.nav-item.router-link-active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 7px;
  bottom: 7px;
  width: 3px;
  border-radius: 3px;
  background: var(--accent);
}
.ext-mark {
  margin-left: auto;
  font-size: 12px;
  opacity: 0.6;
}
.side-foot {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--border);
}
.nav-item.logout:hover {
  color: var(--red);
  background: rgba(239, 68, 68, 0.08);
}
</style>
