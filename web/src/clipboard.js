// 剪贴板工具：优先用 Clipboard API（仅 https/localhost 可用），
// http 环境降级为隐藏 textarea + execCommand，避免弹窗要求手动复制。
export async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // 继续走降级方案
    }
  }
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('readonly', '')
    ta.style.position = 'fixed'
    ta.style.top = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    return ok
  } catch {
    return false
  }
}

// 轻量 toast，替代 alert。
export function toast(msg, ms = 1800) {
  const el = document.createElement('div')
  el.className = 'toast-tip'
  el.textContent = msg
  document.body.appendChild(el)
  requestAnimationFrame(() => el.classList.add('show'))
  setTimeout(() => {
    el.classList.remove('show')
    setTimeout(() => el.remove(), 250)
  }, ms)
}

// 复制并提示。
export async function copyWithToast(text, okMsg = '已复制到剪贴板') {
  const ok = await copyText(text)
  toast(ok ? okMsg : '复制失败，请手动复制')
  return ok
}
