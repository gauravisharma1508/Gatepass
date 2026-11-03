import { apiBase } from '../config'
import { setServerTime } from './serverTime'

export class ApiError extends Error {
  status: number
  code: string
  fields: Record<string, string>
  constructor(status: number, code: string, message: string, fields: Record<string, string> = {}) {
    super(message)
    this.status = status
    this.code = code
    this.fields = fields
  }
}

type Params = Record<string, string | number | boolean | undefined>

let token: string | null = null
let onUnauthorized: () => void = () => {}
export const setToken = (t: string | null) => { token = t }
export const getToken = () => token
export const setUnauthorizedHandler = (fn: () => void) => { onUnauthorized = fn }

export function buildQuery(params?: Params): string {
  if (!params) return ''
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '' || v === false) continue
    sp.set(k, String(v))
  }
  const s = sp.toString()
  return s ? `?${s}` : ''
}

async function request<T>(method: string, path: string, opt: { params?: Params; body?: unknown; noAuthRedirect?: boolean } = {}): Promise<T> {
  const headers: Record<string, string> = {
    'X-Requested-With': 'gatepass', // the login call is not a bearer call, so it still needs this
    'X-Client': 'mobile',
    Accept: 'application/json',
  }
  if (token) headers.Authorization = `Bearer ${token}`
  let body: string | undefined
  if (opt.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opt.body)
  }
  let res: Response
  try {
    res = await fetch(`${apiBase()}/api${path}${buildQuery(opt.params)}`, { method, headers, body })
  } catch {
    throw new ApiError(0, 'NETWORK', 'Cannot reach the server. Check your connection and try again.')
  }
  setServerTime(res.headers.get('Date'))
  if (res.ok) {
    if (res.status === 204 || res.status === 202) return undefined as T
    return (await res.json()) as T
  }
  let parsed: { error?: { code?: string; message?: string; details?: { fields?: Record<string, string> } } } = {}
  try {
    parsed = await res.json()
  } catch {
    /* non-JSON body */
  }
  const e = parsed.error ?? {}
  if (res.status === 401 && !opt.noAuthRedirect) onUnauthorized()
  throw new ApiError(res.status, e.code ?? 'ERROR', e.message ?? `Request failed (${res.status})`, e.details?.fields ?? {})
}

export const api = {
  get: <T>(path: string, params?: Params) => request<T>('GET', path, { params }),
  post: <T = void>(path: string, body?: unknown, noAuthRedirect = false) => request<T>('POST', path, { body, noAuthRedirect }),
  patch: <T>(path: string, body: unknown) => request<T>('PATCH', path, { body }),
}

export const errorMessage = (e: unknown): string => (e instanceof Error ? e.message : 'Something went wrong')
export const fieldErrors = (e: unknown): Record<string, string> | null =>
  e instanceof ApiError && Object.keys(e.fields).length > 0 ? e.fields : null
