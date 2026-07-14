// 亮/暗主题切换：类名挂在 <html> 上，选择记忆到 localStorage。
const KEY = 'probe_theme'

export function getTheme() {
  return localStorage.getItem(KEY) || 'dark'
}

export function applyTheme(theme) {
  document.documentElement.classList.toggle('light', theme === 'light')
}

export function toggleTheme() {
  const next = getTheme() === 'light' ? 'dark' : 'light'
  localStorage.setItem(KEY, next)
  applyTheme(next)
  // 通知需要重绘的组件（如 ECharts 图表）。
  window.dispatchEvent(new CustomEvent('probe-theme'))
  return next
}
