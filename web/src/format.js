// 通用格式化工具。

export function fmtBytes(n) {
  if (!n && n !== 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

export function fmtSpeed(n) {
  return `${fmtBytes(n)}/s`
}

export function fmtUptime(sec) {
  if (!sec) return '-'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d > 0) return `${d}天 ${h}小时`
  if (h > 0) return `${h}小时 ${m}分`
  return `${m}分`
}

export function fmtPercent(v) {
  return `${(v || 0).toFixed(1)}%`
}

export function fmtTime(unix) {
  if (!unix) return '-'
  return new Date(unix * 1000).toLocaleString()
}

// 根据占用百分比返回进度条严重级别 class。
export function barLevel(pct) {
  if (pct >= 90) return 'crit'
  if (pct >= 70) return 'warn'
  return ''
}

// CPU 信息汇总：Agent 上报形如 ["Model 1 Cores", "Model 1 Cores", ...]（KVM 上
// 常见每个虚拟 socket 一条），这里合并同型号并累加核数。
export function cpuSummary(arr) {
  if (!arr || !arr.length) return { text: '', cores: 0 }
  const map = new Map()
  let total = 0
  for (const s of arr) {
    const m = String(s).match(/^(.*?)\s*(\d+)\s+Cores?$/i)
    if (m) {
      const name = m[1].trim()
      const n = Number(m[2])
      map.set(name, (map.get(name) || 0) + n)
      total += n
    } else {
      map.set(String(s), map.get(String(s)) || 0)
    }
  }
  const text = [...map.entries()]
    .map(([name, cores]) => {
      if (!cores) return name
      return name ? `${name} · ${cores} 核` : `${cores} 核`
    })
    .join(' / ')
  return { text, cores: total }
}

// 标签调色板：同一标签文本永远映射到同一颜色。
const TAG_COLORS = [
  '#3b82f6', // blue
  '#22c55e', // green
  '#eab308', // yellow
  '#ef4444', // red
  '#a855f7', // purple
  '#06b6d4', // cyan
  '#f97316', // orange
  '#ec4899', // pink
]

export function tagColor(tag) {
  let h = 0
  for (let i = 0; i < tag.length; i++) {
    h = (h * 31 + tag.charCodeAt(i)) >>> 0
  }
  return TAG_COLORS[h % TAG_COLORS.length]
}
