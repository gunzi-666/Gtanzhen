<script setup>
import { ref, watch, onMounted } from 'vue'
import { fetchHistory } from '../api'
import { fmtBytes, fmtSpeed, fmtUptime, fmtPercent, fmtTime, cpuSummary } from '../format'
import HistoryChart from './HistoryChart.vue'

const props = defineProps({
  server: { type: Object, required: true },
})

const points = ref([])
const metric = ref('cpu')
const hours = ref(1)

async function loadHistory() {
  try {
    points.value = await fetchHistory(props.server.id, hours.value)
  } catch {
    points.value = []
  }
}

watch(() => props.server.id, () => {
  metric.value = 'cpu'
  hours.value = 1
  loadHistory()
})
onMounted(loadHistory)
</script>

<template>
  <div class="card detail-panel">
    <div class="detail-meta muted" v-if="server.host">
      {{ server.host.platform }} · {{ server.host.arch }} ·
      {{ cpuSummary(server.host.cpu).text }}
    </div>
    <div class="detail-grid" v-if="server.metrics">
      <div><span class="muted">CPU</span><b>{{ fmtPercent(server.metrics.cpu) }}</b></div>
      <div><span class="muted">内存</span><b>{{ fmtBytes(server.metrics.mem_used) }}</b></div>
      <div><span class="muted">磁盘</span><b>{{ fmtBytes(server.metrics.disk_used) }}</b></div>
      <div><span class="muted">下行</span><b>{{ fmtSpeed(server.metrics.net_in_speed) }}</b></div>
      <div><span class="muted">上行</span><b>{{ fmtSpeed(server.metrics.net_out_speed) }}</b></div>
      <div><span class="muted">运行</span><b>{{ fmtUptime(server.metrics.uptime) }}</b></div>
      <div><span class="muted">进程</span><b>{{ server.metrics.process_count }}</b></div>
      <div><span class="muted">TCP</span><b>{{ server.metrics.tcp_conn_count }}</b></div>
      <div><span class="muted">负载</span><b>{{ (server.metrics.load1 || 0).toFixed(2) }}</b></div>
      <div><span class="muted">月流量</span><b>↓{{ fmtBytes(server.traffic_in) }} ↑{{ fmtBytes(server.traffic_out) }}</b></div>
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
    <div class="last-seen muted">最后在线：{{ fmtTime(server.last_seen) }}</div>
  </div>
</template>

<style scoped>
.detail-panel {
  cursor: default;
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
@media (max-width: 640px) {
  .detail-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
.detail-grid > div {
  display: flex;
  flex-direction: column;
  background: var(--muted);
  border: 1px solid var(--border);
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
