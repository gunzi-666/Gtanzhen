// 统一的 API 封装，所有请求携带 cookie（credentials: include）。

async function request(url, options = {}) {
  const res = await fetch(url, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const text = await res.text()
  let data = null
  try {
    data = text ? JSON.parse(text) : null
  } catch {
    // 非 JSON 响应
  }
  if (!res.ok) {
    // 后端 401 会带具体原因（unauthorized / status_locked）。
    throw new Error((data && data.error) || res.statusText || 'request failed')
  }
  return data
}

export const api = {
  get: (url) => request(url),
  post: (url, body) => request(url, { method: 'POST', body: JSON.stringify(body) }),
  put: (url, body) => request(url, { method: 'PUT', body: JSON.stringify(body) }),
  del: (url) => request(url, { method: 'DELETE' }),
}

// 公开状态数据。
export const fetchPublicServers = () => api.get('/api/public/servers')
export const fetchHistory = (serverId, hours) =>
  api.get(`/api/public/history?server_id=${serverId}&hours=${hours}`)
export const fetchMonitors = () => api.get('/api/public/monitors')
// 状态页密码解锁。
export const unlockStatus = (password) => api.post('/api/public/unlock', { password })
// 站点外观（站点名/背景图）。
export const fetchSite = () => api.get('/api/public/site')

// 认证。
export const login = (username, password) => api.post('/api/login', { username, password })
export const logout = () => api.post('/api/logout', {})
export async function isLoggedIn() {
  try {
    const r = await api.get('/api/me')
    return !!r.logged_in
  } catch {
    return false
  }
}

// 打开浏览器实时推送 WebSocket。
export function openLiveSocket(onServers) {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const ws = new WebSocket(`${proto}://${location.host}/api/ws`)
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data)
      if (msg.type === 'servers') onServers(msg.data || [])
    } catch {
      // 忽略解析错误
    }
  }
  return ws
}
