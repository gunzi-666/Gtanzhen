<script setup>
import { onMounted, onBeforeUnmount, ref, computed } from 'vue'
import { fetchPublicServers, fetchMonitors, openLiveSocket, unlockStatus, fetchSite } from '../api'
import { fmtBytes, fmtSpeed, fmtUptime, fmtPercent, barLevel, tagColor, cpuSummary } from '../format'
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

// 顶部汇总卡片：台数、在线率、实时总网速、当月总流量。
const summary = computed(() => {
  let inSpeed = 0
  let outSpeed = 0
  let trafficIn = 0
  let trafficOut = 0
  for (const s of servers.value) {
    trafficIn += s.traffic_in || 0
    trafficOut += s.traffic_out || 0
    if (s.online && s.metrics) {
      inSpeed += s.metrics.net_in_speed || 0
      outSpeed += s.metrics.net_out_speed || 0
    }
  }
  const total = servers.value.length
  const online = onlineCount.value
  return {
    total,
    online,
    offline: total - online,
    rate: total ? Math.round((online / total) * 100) : 0,
    inSpeed,
    outSpeed,
    trafficIn,
    trafficOut,
  }
})

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
function osLine(s) {
  return s.host ? `${s.host.platform} · ${s.host.arch}` : '-'
}
function cpuCores(s) {
  return s.host ? cpuSummary(s.host.cpu).cores : 0
}
// 列表列里空间紧凑，速度去掉数字和单位间的空格
function spd(v) {
  return fmtSpeed(v).replace(' ', '')
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
        <!-- 顶部汇总 -->
        <div class="stat-grid">
          <div class="card stat-card">
            <div class="stat-label muted">服务器</div>
            <div class="stat-value">{{ summary.total }}<span class="stat-unit">台</span></div>
            <div class="stat-sub muted">
              <span class="stat-dot" style="background: var(--green)"></span>在线 {{ summary.online }}
              <span class="stat-dot" style="background: var(--red)"></span>离线 {{ summary.offline }}
            </div>
          </div>
          <div class="card stat-card">
            <div class="stat-label muted">在线率</div>
            <div class="stat-value">{{ summary.rate }}<span class="stat-unit">%</span></div>
            <div class="stat-bar">
              <i :style="{ width: summary.rate + '%' }"></i>
            </div>
          </div>
          <div class="card stat-card">
            <div class="stat-label muted">实时网速</div>
            <div class="stat-value">↓ {{ fmtSpeed(summary.inSpeed) }}</div>
            <div class="stat-sub muted">↑ {{ fmtSpeed(summary.outSpeed) }}</div>
          </div>
          <div class="card stat-card">
            <div class="stat-label muted">当月流量</div>
            <div class="stat-value">↓ {{ fmtBytes(summary.trafficIn) }}</div>
            <div class="stat-sub muted">↑ {{ fmtBytes(summary.trafficOut) }}</div>
          </div>
        </div>

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

          <!-- 列表视图：横条卡片，指标分列展示，点击展开详情 -->
          <div v-else class="strip-list">
            <template v-for="s in sec.list" :key="s.id">
              <div class="card srv-strip" :class="{ expanded: expandedId === s.id }" @click="toggle(s)">
                <div class="strip-head">
                  <span class="brand-dot" :style="{ background: s.online ? 'var(--green)' : 'var(--red)' }"></span>
                  <span class="srv-name-text">{{ s.name }}</span>
                  <span v-for="t in (s.tags || [])" :key="t" class="tag hide-sm" :style="tagStyle(t)">{{ t }}</span>
                  <span v-if="s.online && s.metrics" class="strip-right muted hide-sm">{{ osLine(s) }} · 负载 {{ (s.metrics.load1 || 0).toFixed(2) }} · 运行 {{ fmtUptime(s.metrics.uptime) }}</span>
                  <span v-else class="strip-right muted">离线</span>
                </div>
                <div v-if="s.online && s.metrics" class="strip-cols">
                  <div class="scol" :title="cpuCores(s) ? cpuCores(s) + ' 核' : ''">
                    <span class="lbl muted">CPU</span>
                    <span class="val">{{ fmtPercent(s.metrics.cpu) }}</span>
                    <div class="bar mini" :class="barLevel(s.metrics.cpu)"><span :style="{ width: Math.min(s.metrics.cpu, 100) + '%' }"></span></div>
                  </div>
                  <div class="scol" :title="fmtBytes(s.metrics.mem_used) + ' / ' + fmtBytes(s.host && s.host.mem_total)">
                    <span class="lbl muted">内存</span>
                    <span class="val">{{ fmtPercent(memPct(s)) }}</span>
                    <div class="bar mini" :class="barLevel(memPct(s))"><span :style="{ width: Math.min(memPct(s), 100) + '%' }"></span></div>
                  </div>
                  <div class="scol" :title="fmtBytes(s.metrics.disk_used) + ' / ' + fmtBytes(s.host && s.host.disk_total)">
                    <span class="lbl muted">存储</span>
                    <span class="val">{{ fmtPercent(diskPct(s)) }}</span>
                    <div class="bar mini" :class="barLevel(diskPct(s))"><span :style="{ width: Math.min(diskPct(s), 100) + '%' }"></span></div>
                  </div>
                  <div class="scol">
                    <span class="lbl muted">上传</span>
                    <span class="val">{{ spd(s.metrics.net_out_speed) }}</span>
                  </div>
                  <div class="scol">
                    <span class="lbl muted">下载</span>
                    <span class="val">{{ spd(s.metrics.net_in_speed) }}</span>
                  </div>
                  <div class="scol" :title="'↓' + fmtBytes(s.traffic_in) + ' ↑' + fmtBytes(s.traffic_out)">
                    <span class="lbl muted">月流量</span>
                    <span class="val">{{ fmtBytes((s.traffic_in || 0) + (s.traffic_out || 0)) }}</span>
                  </div>
                </div>
                <div v-else class="offline-line muted">暂无实时数据</div>
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

/* 顶部汇总卡片 */
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
  margin-bottom: 18px;
}
@media (max-width: 900px) {
  .stat-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
.stat-card {
  padding: 14px 18px;
}
.stat-label {
  font-size: 12.5px;
  margin-bottom: 6px;
}
.stat-value {
  font-size: 19px;
  font-weight: 700;
  letter-spacing: -0.02em;
  white-space: nowrap;
}
.stat-unit {
  font-size: 12.5px;
  font-weight: 400;
  color: var(--text-dim);
  margin-left: 4px;
}
.stat-sub {
  font-size: 12.5px;
  margin-top: 5px;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}
.stat-dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}
.stat-dot + .stat-dot,
.stat-sub .stat-dot:nth-of-type(2) {
  margin-left: 6px;
}
.stat-bar {
  height: 6px;
  border-radius: 999px;
  background: var(--muted);
  margin-top: 10px;
  overflow: hidden;
}
.stat-bar i {
  display: block;
  height: 100%;
  border-radius: 999px;
  background: var(--green);
  transition: width 0.4s;
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
  font-size: 13.5px;
  font-weight: 600;
  margin: 0 0 12px;
  /* 做成不透明胶囊：自定义背景图（尤其浅色图配深色主题）下文字不会隐形 */
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: var(--card);
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 4px 14px;
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

/* 列表视图：横条卡片 */
.strip-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.srv-strip {
  padding: 12px 16px;
  cursor: pointer;
  transition: border-color 0.15s;
}
.srv-strip:hover,
.srv-strip.expanded {
  border-color: var(--card-border-hover);
}
.strip-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.srv-name-text {
  font-weight: 600;
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.strip-right {
  margin-left: auto;
  flex-shrink: 0;
  font-size: 12px;
  white-space: nowrap;
}
.strip-cols {
  display: flex;
  gap: 10px;
  margin-top: 10px;
}
.scol {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.scol .lbl {
  font-size: 11.5px;
  white-space: nowrap;
}
.scol .val {
  font-size: 12.5px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.scol .bar.mini {
  height: 4px;
  margin-top: 2px;
}
.offline-line {
  font-size: 12.5px;
  margin-top: 8px;
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
.row-detail {
  margin: -2px 0 4px;
  border-radius: var(--radius);
}

@media (max-width: 760px) {
  .hide-sm {
    display: none;
  }
  .srv-strip {
    padding: 11px 12px;
  }
  .strip-cols {
    gap: 8px;
  }
  .scol .val {
    font-size: 12px;
  }
}
</style>
