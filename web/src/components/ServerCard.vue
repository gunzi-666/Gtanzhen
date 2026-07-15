<script setup>
import { computed } from 'vue'
import { fmtBytes, fmtSpeed, fmtUptime, fmtPercent, barLevel, tagColor, cpuSummary } from '../format'

// 标签配色：文字用主色，底/框用半透明。
function tagStyle(t) {
  const c = tagColor(t)
  return { color: c, borderColor: c + '55', background: c + '1f' }
}

const props = defineProps({
  server: { type: Object, required: true },
})
defineEmits(['open'])

const m = computed(() => props.server.metrics || {})
const host = computed(() => props.server.host || {})

const cores = computed(() => cpuSummary(host.value.cpu).cores)

const memPct = computed(() => {
  const t = host.value.mem_total
  return t ? (m.value.mem_used / t) * 100 : 0
})
const diskPct = computed(() => {
  const t = host.value.disk_total
  return t ? (m.value.disk_used / t) * 100 : 0
})
</script>

<template>
  <div class="card server-card" @click="$emit('open', server)">
    <div class="card-head">
      <div class="name">
        <span class="brand-dot" :style="{ background: server.online ? 'var(--green)' : 'var(--red)' }"></span>
        {{ server.name }}
      </div>
      <span class="badge" :class="server.online ? 'online' : 'offline'">
        {{ server.online ? '在线' : '离线' }}
      </span>
    </div>

    <div class="tag-row" v-if="server.tags && server.tags.length">
      <span v-for="t in server.tags" :key="t" class="tag" :style="tagStyle(t)">{{ t }}</span>
    </div>

    <div class="os-line muted" v-if="host.platform">
      {{ host.platform }} · {{ host.arch }}
    </div>

    <template v-if="server.online && m.cpu !== undefined">
      <div class="stat-row"><span>CPU</span><span>{{ fmtPercent(m.cpu) }}<template v-if="cores"> · {{ cores }} 核</template></span></div>
      <div class="bar" :class="barLevel(m.cpu)"><span :style="{ width: Math.min(m.cpu, 100) + '%' }"></span></div>

      <div class="stat-row"><span>内存</span><span>{{ fmtBytes(m.mem_used) }} / {{ fmtBytes(host.mem_total) }}</span></div>
      <div class="bar" :class="barLevel(memPct)"><span :style="{ width: Math.min(memPct, 100) + '%' }"></span></div>

      <div class="stat-row"><span>磁盘</span><span>{{ fmtBytes(m.disk_used) }} / {{ fmtBytes(host.disk_total) }}</span></div>
      <div class="bar" :class="barLevel(diskPct)"><span :style="{ width: Math.min(diskPct, 100) + '%' }"></span></div>

      <div class="net-row">
        <div><span class="muted">↓</span> {{ fmtSpeed(m.net_in_speed) }}</div>
        <div><span class="muted">↑</span> {{ fmtSpeed(m.net_out_speed) }}</div>
      </div>
      <div class="net-row muted small">
        <div>负载 {{ (m.load1 || 0).toFixed(2) }}</div>
        <div>运行 {{ fmtUptime(m.uptime) }}</div>
      </div>
      <div class="net-row muted small" v-if="server.traffic_in || server.traffic_out">
        <div>月流量 ↓{{ fmtBytes(server.traffic_in) }}</div>
        <div>↑{{ fmtBytes(server.traffic_out) }}</div>
      </div>
    </template>
    <div v-else class="offline-hint muted">暂无实时数据</div>
  </div>
</template>

<style scoped>
.server-card {
  cursor: pointer;
}
.card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.name {
  font-weight: 600;
  font-size: 15px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 8px;
}
.os-line {
  font-size: 12px;
  margin: 4px 0 12px;
}
.net-row {
  display: flex;
  justify-content: space-between;
  margin-top: 12px;
}
.net-row.small {
  font-size: 12px;
  margin-top: 6px;
}
.offline-hint {
  padding: 24px 0;
  text-align: center;
}
</style>
