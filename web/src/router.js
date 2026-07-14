import { createRouter, createWebHashHistory } from 'vue-router'
import { isLoggedIn } from './api'

const routes = [
  { path: '/', name: 'status', component: () => import('./views/Status.vue') },
  { path: '/login', name: 'login', component: () => import('./views/Login.vue') },
  {
    path: '/admin',
    component: () => import('./views/admin/AdminLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: '/admin/servers' },
      { path: 'servers', name: 'admin-servers', component: () => import('./views/admin/Servers.vue') },
      { path: 'alerts', name: 'admin-alerts', component: () => import('./views/admin/Alerts.vue') },
      { path: 'notifications', name: 'admin-notifications', component: () => import('./views/admin/Notifications.vue') },
      { path: 'monitors', name: 'admin-monitors', component: () => import('./views/admin/Monitors.vue') },
      { path: 'crons', name: 'admin-crons', component: () => import('./views/admin/Crons.vue') },
    ],
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

// 访问后台前校验登录态。
router.beforeEach(async (to) => {
  if (to.meta.requiresAuth) {
    const ok = await isLoggedIn()
    if (!ok) return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})

export default router
