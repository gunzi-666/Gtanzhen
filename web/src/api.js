// 统一的 API 封装，所有请求携带 cookie（credentials: include）。

async function request(url, options = {}) {
  const res = await fetch(url, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (res.status === 401) {
    throw new Error('unauthorized')
  }
  const text = await res.text()
  const data = text ? JSON.parse(text) : null
  if (!res.ok) {
    throw new Error((data && data.error) || res.statusText)
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
