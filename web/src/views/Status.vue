<script setup>
import { onMounted, onBeforeUnmount, ref, computed } from 'vue'
import { fetchPublicServers, fetchMonitors, openLiveSocket, unlockStatus, fetchSite } from '../api'
import { fmtBytes, fmtSpeed, fmtUptime, fmtPercent, barLevel, tagColor } from '../format'
import { getTheme, toggleTheme } from '../theme'
import ServerCard from '../components/ServerCard.vue'
import ServerDetail from '../components/ServerDetail.vue'

const servers = ref([])
const monitors = ref([])
const expandedId = ref(null)
// 视图模式：card（卡片）/ list（长条列表），记忆到 localStorage。
const viewMode = ref(localStorage.getItem('probe_view_mode') || 'card')
let ws = null

// 站点外观。
const siteName = ref('探针监控')
const background = ref('')
const theme = ref(getTheme())

function switchTheme() {
  theme.value = toggleTheme()
}

// 状态页密码锁。
const locked = ref(false)
const lockPassword = ref('')
const unlocking = ref(false)
const lockError = ref('')

const onlineCount = computed(() => servers.value.filter((s) => s.online).length)

// 分组：有分组时展示筛选 chips；「全部」视图按分组分节显示。
const groups = computed(() => {
  const out = []
  for (const s of servers.value) {
    const g = s.group || ''
    if (g && !out.includes(g)) out.push(g)
  }
  return out
})
const selectedGroup = ref('all')
const sections = computed(() => {
  const of = (g) => servers.value.filter((s) => (s.group || '') === g)
  if (!groups.value.length) return [{ name: '', list: servers.value }]
  if (selectedGroup.value !== 'all') {
    return [{ name: '', list: of(selectedGroup.value === '__none' ? '' : selectedGroup.value) }]
  }
  const secs = groups.value.map((g) => ({ name: g, list: of(g) }))
  const rest = of('')
  if (rest.length) secs.push({ name: '未分组', list: rest })
  return secs
})

function setView(mode) {
  viewMode.value = mode
  localStorage.setItem('probe_view_mode', mode)
}

// 点击卡片/行：展开或收起详情。
function toggle(s) {
  expandedId.value = expandedId.value === s.id ? null : s.id
}

function memPct(s) {
  const t = s.host && s.host.mem_total
  return t ? ((s.metrics?.mem_used || 0) / t) * 100 : 0
}
function diskPct(s) {
  const t = s.host && s.host.disk_total
  return t ? ((s.metrics?.disk_used || 0) / t) * 100 : 0
}

function tagStyle(t) {
  const c = tagColor(t)
  return { color: c, borderColor: c + '55', background: c + '1f' }
}

async function loadMonitors() {
  try {
    monitors.value = await fetchMonitors()
  } catch {
    monitors.value = []
  }
}

async function loadAll() {
  try {
    servers.value = await fetchPublicServers()
    locked.value = false
  } catch (e) {
    if (e.message === 'status_locked') {
      locked.value = true
      return
    }
  }
  loadMonitors()
  ws && ws.close()
  ws = openLiveSocket((data) => {
    servers.value = data
  })
}

async function doUnlock() {
  if (!lockPassword.value) return
  unlocking.value = true
  lockError.value = ''
  try {
    await unlockStatus(lockPassword.value)
    lockPassword.value = ''
    await loadAll()
  } catch (e) {
    lockError.value = e.message || '解锁失败'
  } finally {
    unlocking.value = false
  }
}

async function loadSite() {
  try {
    const s = await fetchSite()
    if (s.site_name) {
      siteName.value = s.site_name
      document.title = s.site_name
    }
    background.value = s.status_background || ''
  } catch {
    // 用默认值
  }
}

let monitorTimer = null
onMounted(async () => {
  loadSite()
  await loadAll()
  monitorTimer = setInterval(() => {
    if (!locked.value) loadMonitors()
  }, 15000)
})
onBeforeUnmount(() => {
  ws && ws.close()
  monitorTimer && clearInterval(monitorTimer)
})

const typeLabel = { ping: 'Ping', tcping: 'TCP', http_get: 'HTTP' }
</script>

<template>
  <div>
    <!-- 自定义背景图（含遮罩保证可读性） -->
    <div v-if="background" class="bg-layer" :style="{ backgroundImage: `url(${background})` }"></div>

    <header class="topbar">
      <h1><span class="brand-dot"></span> {{ siteName }}</h1>
      <div class="head-right">
        <div class="seg">
          <button :class="{ active: viewMode === 'card' }" @click="setView('card')">卡片</button>
          <button :class="{ active: viewMode === 'list' }" @click="setView('list')">列表</button>
        </div>
        <button class="ghost theme-btn" :title="theme === 'light' ? '切换暗色' : '切换亮色'" @click="switchTheme">
          {{ theme === 'light' ? '☾' : '☀' }}
        </button>
        <span class="chip">{{ onlineCount }} / {{ servers.length }} 在线</span>
        <RouterLink to="/admin" class="admin-link">管理后台</RouterLink>
      </div>
    </header>

    <!-- 状态页密码锁 -->
    <div v-if="locked" class="lock-wrap">
      <div class="card lock-card">
        <h3>此状态页需要密码访问</h3>
        <p class="muted">请输入访问密码继续。</p>
        <input v-model="lockPassword" type="password" placeholder="访问密码" @keyup.enter="doUnlock" />
        <div v-if="lockError" class="lock-error">{{ lockError }}</div>
        <button :disabled="unlocking" @click="doUnlock">{{ unlocking ? '验证中...' : '进入' }}</button>
      </div>
    </div>

    <div v-else class="container">
      <div v-if="servers.length === 0" class="empty muted">
        暂无服务器。请到管理后台添加。
      </div>

      <template v-else>
        <!-- 分组筛选 -->
        <div v-if="groups.length" class="group-chips">
          <button class="group-chip" :class="{ active: selectedGroup === 'all' }" @click="selectedGroup = 'all'">全部</button>
          <button v-for="g in groups" :key="g" class="group-chip" :class="{ active: selectedGroup === g }" @click="selectedGroup = g">{{ g }}</button>
          <button v-if="servers.some((s) => !s.group)" class="group-chip" :class="{ active: selectedGroup === '__none' }" @click="selectedGroup = '__none'">未分组</button>
        </div>

        <section v-for="sec in sections" :key="sec.name || '_'" class="group-section">
          <h2 v-if="sec.name" class="group-title">{{ sec.name }} <span class="muted group-count">{{ sec.list.length }}</span></h2>

          <!-- 卡片视图：点击卡片在下方整行展开详情 -->
          <div v-if="viewMode === 'card'" class="grid">
            <template v-for="s in sec.list" :key="s.id">
              <ServerCard :server="s" :class="{ expanded: expandedId === s.id }" @open="toggle" />
              <ServerDetail v-if="expandedId === s.id" :server="s" class="detail-span" />
            </template>
          </div>

          <!-- 列表视图：长条行，点击展开详情 -->
          <div v-else class="card list-wrap">
            <div class="srv-row srv-head muted">
              <div>名称</div>
              <div class="hide-sm">系统</div>
              <div>CPU</div>
              <div>内存</div>
              <div class="hide-sm">磁盘</div>
              <div class="hide-sm">网络</div>
              <div class="hide-sm">运行</div>
            </div>
            <template v-for="s in sec.list" :key="s.id">
              <div class="srv-row" :class="{ expanded: expandedId === s.id }" @click="toggle(s)">
                <div class="srv-name">
                  <span class="brand-dot" :style="{ background: s.online ? 'var(--green)' : 'var(--red)' }"></span>
                  <span class="srv-name-text">{{ s.name }}</span>
                  <span v-for="t in (s.tags || [])" :key="t" class="tag hide-sm" :style="tagStyle(t)">{{ t }}</span>
                </div>
                <div class="muted hide-sm srv-os">{{ s.host ? `${s.host.platform} · ${s.host.arch}` : '-' }}</div>
                <template v-if="s.online && s.metrics">
                  <div class="cell-bar">
                    <span class="pct">{{ fmtPercent(s.metrics.cpu) }}</span>
                    <div class="bar" :class="barLevel(s.metrics.cpu)"><span :style="{ width: Math.min(s.metrics.cpu, 100) + '%' }"></span></div>
                  </div>
                  <div class="cell-bar">
                    <span class="pct">{{ fmtPercent(memPct(s)) }}</span>
                    <div class="bar" :class="barLevel(memPct(s))"><span :style="{ width: Math.min(memPct(s), 100) + '%' }"></span></div>
                  </div>
                  <div class="cell-bar hide-sm">
                    <span class="pct">{{ fmtPercent(diskPct(s)) }}</span>
                    <div class="bar" :class="barLevel(diskPct(s))"><span :style="{ width: Math.min(diskPct(s), 100) + '%' }"></span></div>
                  </div>
                  <div class="muted hide-sm srv-net">↓{{ fmtSpeed(s.metrics.net_in_speed) }} ↑{{ fmtSpeed(s.metrics.net_out_speed) }}</div>
                  <div class="muted hide-sm">{{ fmtUptime(s.metrics.uptime) }}</div>
                </template>
                <template v-else>
                  <div class="muted offline-cell">离线</div>
                  <div></div>
                  <div class="hide-sm"></div>
                  <div class="hide-sm"></div>
                  <div class="hide-sm"></div>
                </template>
              </div>
              <ServerDetail v-if="expandedId === s.id" :server="s" class="row-detail" @click.stop />
            </template>
          </div>
        </section>
      </template>

      <template v-if="monitors.length > 0">
        <h2 class="section-title">服务可用性</h2>
        <div class="monitor-grid">
          <div v-for="mo in monitors" :key="mo.id" class="card monitor-item">
            <div class="mo-head">
              <span class="brand-dot" :style="{ background: !mo.has_result ? 'var(--text-dim)' : (mo.last_success ? 'var(--green)' : 'var(--red)') }"></span>
              <b>{{ mo.name }}</b>
              <span class="chip">{{ typeLabel[mo.type] || mo.type }}</span>
            </div>
            <div class="mo-target muted">{{ mo.target }}</div>
            <div class="mo-status">
              <span v-if="!mo.has_result" class="muted">等待首次探测</span>
              <span v-else-if="mo.last_success" class="ok">正常 · {{ mo.last_delay.toFixed(0) }}ms</span>
              <span v-else class="fail">异常 · {{ mo.last_message || '探测失败' }}</span>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.head-right {
  display: flex;
  align-items: center;
  gap: 14px;
}

/* 自定义背景图（原样显示，不加遮罩） */
.bg-layer {
  position: fixed;
  inset: 0;
  z-index: -1;
  background-size: cover;
  background-position: center;
}

.theme-btn {
  padding: 4px 10px;
  font-size: 14px;
  line-height: 1.2;
}

/* 分组筛选与分节 */
.group-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 18px;
}
.group-chip {
  background: var(--muted);
  border: 1px solid var(--border);
  color: var(--text-dim);
  padding: 3px 14px;
  font-size: 12.5px;
  border-radius: 999px;
}
.group-chip:hover {
  color: var(--text);
  opacity: 1;
}
.group-chip.active {
  background: var(--primary);
  border-color: var(--primary);
  color: var(--primary-fg);
}
.group-section + .group-section {
  margin-top: 26px;
}
.group-title {
  font-size: 15px;
  font-weight: 600;
  margin: 0 0 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.group-count {
  font-size: 12px;
  font-weight: 400;
}
.admin-link {
  color: var(--text-dim);
}
.empty {
  text-align: center;
  padding: 80px 0;
}
.section-title {
  margin: 32px 0 16px;
  font-size: 18px;
}
.monitor-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 12px;
}
.mo-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.mo-target {
  font-size: 12px;
  margin: 6px 0 8px;
  word-break: break-all;
}
.mo-status .ok {
  color: var(--green);
}
.mo-status .fail {
  color: var(--red);
}

/* 卡片视图：展开详情占满整行 */
.detail-span {
  grid-column: 1 / -1;
}
.grid :deep(.server-card.expanded) {
  border-color: var(--card-border-hover);
}

/* 列表视图 */
.list-wrap {
  padding: 4px 8px;
}
.srv-row {
  display: grid;
  grid-template-columns: 1.3fr 1fr 1fr 1fr 1fr 1.1fr 0.7fr;
  gap: 12px;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
  border-radius: 6px;
  font-size: 13.5px;
  transition: background 0.1s;
}
.srv-row:hover {
  background: var(--hover);
}
.srv-row.srv-head {
  cursor: default;
  font-size: 12.5px;
  border-bottom: 1px solid var(--border);
}
.srv-row.srv-head:hover {
  background: transparent;
}
.srv-row.expanded {
  background: var(--hover);
}
.srv-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
}
.srv-name-text {
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 密码锁 */
.lock-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: calc(100vh - 120px);
}
.lock-card {
  width: 360px;
  max-width: 90vw;
  text-align: center;
  padding: 32px 28px;
}
.lock-card h3 {
  margin: 0 0 6px;
}
.lock-card .muted {
  font-size: 13px;
  margin: 0 0 18px;
}
.lock-card input {
  margin-bottom: 12px;
  text-align: center;
}
.lock-card button {
  width: 100%;
}
.lock-error {
  color: var(--red);
  font-size: 13px;
  margin-bottom: 10px;
}
.srv-os,
.srv-net {
  font-size: 12.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cell-bar {
  display: flex;
  align-items: center;
  gap: 8px;
}
.cell-bar .pct {
  font-size: 12px;
  width: 44px;
  flex-shrink: 0;
  text-align: right;
}
.cell-bar .bar {
  flex: 1;
}
.offline-cell {
  font-size: 12.5px;
}
.row-detail {
  margin: 8px 4px 12px;
  border-radius: var(--radius);
}

@media (max-width: 760px) {
  .srv-row {
    grid-template-columns: 1.2fr 1fr 1fr;
  }
  .hide-sm {
    display: none;
  }
}
</style>
