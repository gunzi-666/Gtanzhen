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
