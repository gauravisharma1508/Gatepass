import { setServerTime } from './serverTime'

export interface ApiErrorBody {
  error?: { code?: string; message?: string; details?: { fields?: Record<string, string> } }
}

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

type UnauthorizedHandler = () => void
let onUnauthorized: UnauthorizedHandler = () => {}
export function setUnauthorizedHandler(fn: UnauthorizedHandler) {
  onUnauthorized = fn
}

const BASE = '/api'

function query(params?: Record<string, string | number | boolean | undefined | string[]>): string {
  if (!params) return ''
  const sp = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '' || v === false) continue
    if (Array.isArray(v)) v.forEach((x) => sp.append(k, x))
    else sp.set(k, String(v))
  }
  const s = sp.toString()
  return s ? `?${s}` : ''
}

export function apiUrl(path: string, params?: Record<string, string | number | boolean | undefined | string[]>): string {
  return `${BASE}${path}${query(params)}`
}

interface Options {
  params?: Record<string, string | number | boolean | undefined | string[]>
  body?: unknown
  form?: FormData
  /** do not treat 401 as "session ended" (login form itself) */
  noAuthRedirect?: boolean
}

async function request<T>(method: string, path: string, opt: Options = {}): Promise<T> {
  const headers: Record<string, string> = { 'X-Requested-With': 'gatepass', Accept: 'application/json' }
  let body: BodyInit | undefined
  if (opt.form) body = opt.form
  else if (opt.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opt.body)
  }
  let res: Response
  try {
    res = await fetch(apiUrl(path, opt.params), { method, headers, body, credentials: 'include' })
  } catch {
    throw new ApiError(0, 'NETWORK', 'Cannot reach the server. Check your connection and try again.')
  }
  setServerTime(res.headers.get('Date'))
  if (res.ok) {
    if (res.status === 204 || res.status === 202) return undefined as T
    const ct = res.headers.get('Content-Type') ?? ''
    if (!ct.includes('json')) return undefined as T
    return (await res.json()) as T
  }
  let parsed: ApiErrorBody = {}
  try {
    parsed = (await res.json()) as ApiErrorBody
  } catch {
    /* non-JSON error body */
  }
  const e = parsed.error ?? {}
  const err = new ApiError(res.status, e.code ?? 'ERROR', e.message ?? `Request failed (${res.status})`, e.details?.fields ?? {})
  if (res.status === 401 && !opt.noAuthRedirect) onUnauthorized()
  throw err
}

export const api = {
  get: <T>(path: string, params?: Options['params'], opt?: Pick<Options, 'noAuthRedirect'>) =>
    request<T>('GET', path, { params, ...opt }),
  post: <T = void>(path: string, body?: unknown, opt?: Pick<Options, 'noAuthRedirect'>) =>
    request<T>('POST', path, { body, ...opt }),
  patch: <T>(path: string, body: unknown) => request<T>('PATCH', path, { body }),
  put: <T = void>(path: string, form: FormData) => request<T>('PUT', path, { form }),
  upload: <T = void>(path: string, form: FormData) => request<T>('POST', path, { form }),
}

export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : 'Something went wrong'
}
