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

// 当前计费周期总流量（下行+上行）。
const trafficTotal = computed(() => (props.server.traffic_in || 0) + (props.server.traffic_out || 0))

// 速率紧凑显示：去掉空格省宽度（11.5 KB/s -> 11.5KB/s）。
function spd(v) {
  return fmtSpeed(v).replace(' ', '')
}
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

    <template v-if="server.online && m.cpu !== undefined">
      <!-- 指标列：标签在上、数值在下，百分比类带迷你进度条 -->
      <div class="metric-cols">
        <div class="mcol" :title="cores ? cores + ' 核' : ''">
          <span class="lbl muted">CPU</span>
          <span class="val">{{ fmtPercent(m.cpu) }}</span>
          <div class="bar mini" :class="barLevel(m.cpu)"><span :style="{ width: Math.min(m.cpu, 100) + '%' }"></span></div>
        </div>
        <div class="mcol" :title="fmtBytes(m.mem_used) + ' / ' + fmtBytes(host.mem_total)">
          <span class="lbl muted">内存</span>
          <span class="val">{{ fmtPercent(memPct) }}</span>
          <div class="bar mini" :class="barLevel(memPct)"><span :style="{ width: Math.min(memPct, 100) + '%' }"></span></div>
        </div>
        <div class="mcol" :title="fmtBytes(m.disk_used) + ' / ' + fmtBytes(host.disk_total)">
          <span class="lbl muted">存储</span>
          <span class="val">{{ fmtPercent(diskPct) }}</span>
          <div class="bar mini" :class="barLevel(diskPct)"><span :style="{ width: Math.min(diskPct, 100) + '%' }"></span></div>
        </div>
        <div class="mcol">
          <span class="lbl muted">上传</span>
          <span class="val">{{ spd(m.net_out_speed) }}</span>
        </div>
        <div class="mcol">
          <span class="lbl muted">下载</span>
          <span class="val">{{ spd(m.net_in_speed) }}</span>
        </div>
        <div class="mcol" :title="'↓' + fmtBytes(server.traffic_in) + ' ↑' + fmtBytes(server.traffic_out)">
          <span class="lbl muted">月流量</span>
          <span class="val">{{ fmtBytes(trafficTotal) }}</span>
        </div>
      </div>

      <div class="foot-row muted">
        <span v-if="host.platform">{{ host.platform }} · {{ host.arch }}<template v-if="cores"> · {{ cores }}核</template></span>
        <span>负载 {{ (m.load1 || 0).toFixed(2) }} · 运行 {{ fmtUptime(m.uptime) }}</span>
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
  min-width: 0;
}
.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 8px;
}

.metric-cols {
  display: flex;
  gap: 8px;
  margin-top: 14px;
}
.mcol {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.lbl {
  font-size: 11.5px;
  white-space: nowrap;
}
.val {
  font-size: 12.5px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.bar.mini {
  height: 4px;
  margin-top: 2px;
}

.foot-row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  font-size: 11.5px;
  margin-top: 12px;
  white-space: nowrap;
  overflow: hidden;
}
.foot-row span {
  overflow: hidden;
  text-overflow: ellipsis;
}
.offline-hint {
  padding: 24px 0;
  text-align: center;
}
</style>
