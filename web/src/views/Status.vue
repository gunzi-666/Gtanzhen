<script setup>
import { onMounted, onBeforeUnmount, ref, computed } from 'vue'
import { fetchPublicServers, fetchMonitors, openLiveSocket } from '../api'
import { fmtBytes, fmtSpeed, fmtUptime, fmtPercent, barLevel } from '../format'
import ServerCard from '../components/ServerCard.vue'
import ServerDetail from '../components/ServerDetail.vue'

const servers = ref([])
const monitors = ref([])
const expandedId = ref(null)
// 视图模式：card（卡片）/ list（长条列表），记忆到 localStorage。
const viewMode = ref(localStorage.getItem('probe_view_mode') || 'card')
let ws = null

const onlineCount = computed(() => servers.value.filter((s) => s.online).length)

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

async function loadMonitors() {
  try {
    monitors.value = await fetchMonitors()
  } catch {
    monitors.value = []
  }
}

let monitorTimer = null
onMounted(async () => {
  try {
    servers.value = await fetchPublicServers()
  } catch {
    // ignore
  }
  loadMonitors()
  monitorTimer = setInterval(loadMonitors, 15000)
  ws = openLiveSocket((data) => {
    servers.value = data
  })
})
onBeforeUnmount(() => {
  ws && ws.close()
  monitorTimer && clearInterval(monitorTimer)
})

const typeLabel = { ping: 'Ping', tcping: 'TCP', http_get: 'HTTP' }
</script>

<template>
  <div>
    <header class="topbar">
      <h1><span class="brand-dot"></span> 探针监控</h1>
      <div class="head-right">
        <div class="seg">
          <button :class="{ active: viewMode === 'card' }" @click="setView('card')">卡片</button>
          <button :class="{ active: viewMode === 'list' }" @click="setView('list')">列表</button>
        </div>
        <span class="chip">{{ onlineCount }} / {{ servers.length }} 在线</span>
        <RouterLink to="/admin" class="admin-link">管理后台</RouterLink>
      </div>
    </header>

    <div class="container">
      <div v-if="servers.length === 0" class="empty muted">
        暂无服务器。请到管理后台添加。
      </div>

      <!-- 卡片视图：点击卡片在下方整行展开详情 -->
      <div v-else-if="viewMode === 'card'" class="grid">
        <template v-for="s in servers" :key="s.id">
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
        <template v-for="s in servers" :key="s.id">
          <div class="srv-row" :class="{ expanded: expandedId === s.id }" @click="toggle(s)">
            <div class="srv-name">
              <span class="brand-dot" :style="{ background: s.online ? 'var(--green)' : 'var(--red)' }"></span>
              {{ s.name }}
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
  border-color: #4b4b52;
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
  background: rgba(255, 255, 255, 0.03);
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
  background: rgba(255, 255, 255, 0.03);
}
.srv-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
