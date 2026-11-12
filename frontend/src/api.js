import router from './router'

// 发送API请求
export async function api(path, options = {}) {
  const headers = { ...(options.body ? { 'Content-Type': 'application/json' } : {}), ...options.headers }
  const token = localStorage.getItem('analytics_token')
  if (token) headers.Authorization = `Bearer ${token}`
  const response = await fetch(`/api${path}`, { ...options, headers })
  if (response.status === 401) { localStorage.removeItem('analytics_token'); if (router.currentRoute.value.path !== '/login') router.push('/login') }
  const contentType = response.headers.get('content-type') || ''
  const payload = contentType.includes('application/json') ? await response.json() : await response.text()
  if (!response.ok) throw new Error(payload?.message || payload || '请求失败')
  return payload
}

// 构建查询字符串
export function queryString(filters = {}) {
  const params = new URLSearchParams()
  Object.entries(filters).forEach(([key, value]) => { if (value !== '' && value !== null && value !== undefined) params.set(key, value) })
  const value = params.toString()
  return value ? `?${value}` : ''
}
