const API_BASE = (import.meta.env.VITE_API_BASE_URL || '/api/v1').replace(/\/$/, '')
const WS_BASE = import.meta.env.VITE_WS_BASE_URL || defaultWsBase()

function defaultWsBase() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}/api/v1/ws`
}

export async function apiRequest(path, options = {}) {
  const {
    method = 'GET',
    body,
    token = '',
    headers = {}
  } = options

  const requestHeaders = { ...headers }
  const requestInit = {
    method,
    headers: requestHeaders
  }

  if (token) {
    requestHeaders.Authorization = `Bearer ${token}`
  }

  if (body instanceof FormData) {
    requestInit.body = body
  } else if (body !== undefined && body !== null) {
    requestHeaders['Content-Type'] = 'application/json'
    requestInit.body = JSON.stringify(body)
  }

  const response = await fetch(`${API_BASE}${path}`, requestInit)
  const rawText = await response.text()
  const contentType = response.headers.get('content-type') || ''
  let payload = null

  if (rawText) {
    if (contentType.includes('application/json')) {
      try {
        payload = JSON.parse(rawText)
      } catch {
        payload = null
      }
    } else {
      payload = { raw: rawText }
    }
  }

  if (!response.ok) {
    let message = payload?.error?.message || ''

    if (!message && response.status === 413) {
      message = '上传文件过大，已被网关拒绝。请检查文件大小是否超过 10MB。'
    }

    if (!message && payload?.raw) {
      message = payload.raw.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim()
    }

    if (!message) {
      message = `请求失败 (${response.status})`
    }

    throw new Error(message)
  }

  return payload?.data ?? payload
}

export function buildWsUrl(token) {
  const url = new URL(WS_BASE, window.location.origin)
  if (token) {
    url.searchParams.set('access_token', token)
  }
  return url.toString()
}

export function formatTime(value) {
  if (!value) {
    return '--:--'
  }

  const date = new Date(value)
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date)
}

export function shortId(value) {
  if (!value) {
    return '未知用户'
  }
  return `${value.slice(0, 6)}...${value.slice(-4)}`
}

export function renderMessageHtml(content) {
  const escaped = (content || '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')

  return escaped
    .replace(
      /(https?:\/\/[^\s]+)/g,
      '<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>'
    )
    .replaceAll('\n', '<br />')
}
