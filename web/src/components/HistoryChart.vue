<script setup>
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { fmtBytes } from '../format'

// 仅按需注册用到的图表与组件，显著减小打包体积。
echarts.use([LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

const props = defineProps({
  points: { type: Array, default: () => [] },
  metric: { type: String, default: 'cpu' }, // cpu | mem | net
})

const el = ref(null)
let chart = null

function render() {
  if (!chart) return
  const pts = props.points || []
  const times = pts.map((p) => new Date(p.ts * 1000).toLocaleTimeString())

  let series = []
  let yFmt = (v) => v
  if (props.metric === 'cpu') {
    series = [{ name: 'CPU %', type: 'line', smooth: true, showSymbol: false, areaStyle: {}, data: pts.map((p) => p.cpu.toFixed(1)) }]
  } else if (props.metric === 'mem') {
    yFmt = fmtBytes
    series = [{ name: '内存', type: 'line', smooth: true, showSymbol: false, areaStyle: {}, data: pts.map((p) => p.mem_used) }]
  } else {
    yFmt = (v) => fmtBytes(v) + '/s'
    series = [
      { name: '入站', type: 'line', smooth: true, showSymbol: false, data: pts.map((p) => p.net_in) },
      { name: '出站', type: 'line', smooth: true, showSymbol: false, data: pts.map((p) => p.net_out) },
    ]
  }

  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', valueFormatter: yFmt },
    legend: { show: props.metric === 'net', textStyle: { color: '#9aa3b2' } },
    grid: { left: 60, right: 20, top: 30, bottom: 30 },
    xAxis: { type: 'category', data: times, axisLine: { lineStyle: { color: '#2a2f3a' } }, axisLabel: { color: '#9aa3b2' } },
    yAxis: { type: 'value', axisLabel: { color: '#9aa3b2', formatter: yFmt }, splitLine: { lineStyle: { color: '#2a2f3a' } } },
    series,
    color: ['#4f8cff', '#35c46a'],
  }, true)
}

onMounted(() => {
  chart = echarts.init(el.value)
  render()
  window.addEventListener('resize', resize)
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', resize)
  chart && chart.dispose()
})
function resize() {
  chart && chart.resize()
}
watch(() => [props.points, props.metric], render, { deep: true })
</script>

<template>
  <div ref="el" class="chart"></div>
</template>

<style scoped>
.chart {
  width: 100%;
  height: 260px;
}
</style>
