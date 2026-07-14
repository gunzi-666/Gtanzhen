<script setup>
import { onMounted, onBeforeUnmount, ref, computed } from 'vue'
import { fetchPublicServers, fetchHistory, fetchMonitors, openLiveSocket } from '../api'
import { fmtBytes, fmtSpeed, fmtUptime, fmtPercent, fmtTime } from '../format'
import ServerCard from '../components/ServerCard.vue'
import HistoryChart from '../components/HistoryChart.vue'

const servers = ref([])
const monitors = ref([])
const detail = ref(null)
const points = ref([])
const metric = ref('cpu')
const hours = ref(1)
let ws = null

const onlineCount = computed(() => servers.value.filter((s) => s.online).length)

async function loadHistory() {
  if (!detail.value) return
  try {
    points.value = await fetchHistory(detail.value.id, hours.value)
  } catch {
    points.value = []
  }
}

function openDetail(s) {
  detail.value = s
  metric.value = 'cpu'
  hours.value = 1
  loadHistory()
}
function closeDetail() {
  detail.value = null
  points.value = []
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
    // 同步详情弹窗里的实时数据。
    if (detail.value) {
      const found = data.find((s) => s.id === detail.value.id)
      if (found) detail.value = { ...detail.value, ...found }
    }
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
        <span class="chip">{{ onlineCount }} / {{ servers.length }} 在线</span>
        <RouterLink to="/admin" class="admin-link">管理后台</RouterLink>
      </div>
    </header>

    <div class="container">
      <div v-if="servers.length === 0" class="empty muted">
        暂无服务器。请到管理后台添加。
      </div>
      <div v-else class="grid">
        <ServerCard v-for="s in servers" :key="s.id" :server="s" @open="openDetail" />
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

    <div v-if="detail" class="modal-mask" @click.self="closeDetail">
      <div class="modal detail-modal">
        <div class="page-head">
          <h3>{{ detail.name }}</h3>
          <button class="ghost small" @click="closeDetail">关闭</button>
        </div>

        <div class="detail-meta muted" v-if="detail.host">
          {{ detail.host.platform }} · {{ detail.host.arch }} ·
          {{ (detail.host.cpu && detail.host.cpu[0]) || '' }}
        </div>
        <div class="detail-grid" v-if="detail.metrics">
          <div><span class="muted">CPU</span><b>{{ fmtPercent(detail.metrics.cpu) }}</b></div>
          <div><span class="muted">内存</span><b>{{ fmtBytes(detail.metrics.mem_used) }}</b></div>
          <div><span class="muted">磁盘</span><b>{{ fmtBytes(detail.metrics.disk_used) }}</b></div>
          <div><span class="muted">下行</span><b>{{ fmtSpeed(detail.metrics.net_in_speed) }}</b></div>
          <div><span class="muted">上行</span><b>{{ fmtSpeed(detail.metrics.net_out_speed) }}</b></div>
          <div><span class="muted">运行</span><b>{{ fmtUptime(detail.metrics.uptime) }}</b></div>
          <div><span class="muted">进程</span><b>{{ detail.metrics.process_count }}</b></div>
          <div><span class="muted">TCP</span><b>{{ detail.metrics.tcp_conn_count }}</b></div>
        </div>

        <div class="chart-controls">
          <div class="tabs">
            <button :class="{ ghost: metric !== 'cpu' }" class="small" @click="metric = 'cpu'">CPU</button>
            <button :class="{ ghost: metric !== 'mem' }" class="small" @click="metric = 'mem'">内存</button>
            <button :class="{ ghost: metric !== 'net' }" class="small" @click="metric = 'net'">网络</button>
          </div>
          <select v-model.number="hours" @change="loadHistory" class="hours-select">
            <option :value="1">近 1 小时</option>
            <option :value="6">近 6 小时</option>
            <option :value="24">近 24 小时</option>
            <option :value="168">近 7 天</option>
          </select>
        </div>

        <HistoryChart :points="points" :metric="metric" />
        <div v-if="points.length === 0" class="muted chart-empty">暂无历史数据（需运行一段时间后聚合）</div>
        <div class="last-seen muted">最后在线：{{ fmtTime(detail.last_seen) }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.head-right {
  display: flex;
  align-items: center;
  gap: 16px;
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
.detail-modal {
  width: 720px;
}
.detail-meta {
  font-size: 13px;
  margin-bottom: 14px;
}
.detail-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 20px;
}
.detail-grid > div {
  display: flex;
  flex-direction: column;
  background: var(--bg-soft);
  border-radius: 8px;
  padding: 10px;
}
.detail-grid b {
  margin-top: 4px;
}
.chart-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.tabs {
  display: flex;
  gap: 8px;
}
.hours-select {
  width: auto;
}
.chart-empty {
  text-align: center;
  margin-top: -140px;
  margin-bottom: 120px;
}
.last-seen {
  font-size: 12px;
  margin-top: 12px;
  text-align: right;
}
</style>
