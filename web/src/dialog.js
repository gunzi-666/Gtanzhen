// 风格化对话框，替代原生 confirm()/alert()，与后台 shadcn 风格一致。
// 纯 DOM 实现，任何页面（含非 Vue 上下文）都能直接调用。

function buildModal({ title, message, buttons }) {
  const mask = document.createElement('div')
  mask.className = 'modal-mask'

  const modal = document.createElement('div')
  modal.className = 'modal confirm-modal'

  const h = document.createElement('h3')
  h.textContent = title
  modal.appendChild(h)

  const p = document.createElement('p')
  p.className = 'confirm-msg'
  p.textContent = message
  modal.appendChild(p)

  const actions = document.createElement('div')
  actions.className = 'actions'
  for (const b of buttons) actions.appendChild(b)
  modal.appendChild(actions)

  mask.appendChild(modal)
  return mask
}

// confirmDialog 居中确认框，返回 Promise<boolean>。
// 支持 danger: true 时确认键使用危险配色（删除类操作）。
export function confirmDialog(message, { title = '确认操作', okText = '确定', cancelText = '取消', danger = false } = {}) {
  return new Promise((resolve) => {
    const btnCancel = document.createElement('button')
    btnCancel.className = 'ghost'
    btnCancel.textContent = cancelText

    const btnOk = document.createElement('button')
    if (danger) btnOk.className = 'danger'
    btnOk.textContent = okText

    const mask = buildModal({ title, message, buttons: [btnCancel, btnOk] })

    const close = (val) => {
      document.removeEventListener('keydown', onKey)
      mask.remove()
      resolve(val)
    }
    const onKey = (e) => {
      if (e.key === 'Escape') close(false)
      if (e.key === 'Enter') close(true)
    }

    btnCancel.addEventListener('click', () => close(false))
    btnOk.addEventListener('click', () => close(true))
    mask.addEventListener('click', (e) => {
      if (e.target === mask) close(false)
    })
    document.addEventListener('keydown', onKey)

    document.body.appendChild(mask)
    btnOk.focus()
  })
}

// alertDialog 只有一个确定键的提示框，用于跳转前必须让用户看到的消息。
export function alertDialog(message, { title = '提示', okText = '确定' } = {}) {
  return new Promise((resolve) => {
    const btnOk = document.createElement('button')
    btnOk.textContent = okText

    const mask = buildModal({ title, message, buttons: [btnOk] })

    const close = () => {
      document.removeEventListener('keydown', onKey)
      mask.remove()
      resolve()
    }
    const onKey = (e) => {
      if (e.key === 'Escape' || e.key === 'Enter') close()
    }

    btnOk.addEventListener('click', close)
    document.addEventListener('keydown', onKey)

    document.body.appendChild(mask)
    btnOk.focus()
  })
}
