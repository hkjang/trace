export class APIError extends Error {
  status: number
  code: string
  constructor(message: string, status: number, code = 'request_failed') {
    super(message)
    this.status = status
    this.code = code
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  if (init.method && init.method !== 'GET') headers.set('X-Trace-Request', '1')
  const response = await fetch(path, { ...init, headers, credentials: 'same-origin' })
  if (!response.ok) {
    const value = await response.json().catch(() => ({})) as { error?: { code?: string; message?: string } }
    throw new APIError(value.error?.message || `요청이 실패했습니다 (${response.status})`, response.status, value.error?.code)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export function formatDate(value?: string, includeTime = false) {
  if (!value) return '—'
  return new Intl.DateTimeFormat('ko-KR', { year: 'numeric', month: 'short', day: 'numeric', ...(includeTime ? { hour: '2-digit', minute: '2-digit' } : {}) }).format(new Date(value))
}

export function toLocalDateTime(date = new Date()) {
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}
