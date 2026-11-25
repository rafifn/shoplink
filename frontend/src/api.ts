/** How long the home page waits for /healthz before calling the API unreachable. */
export const HEALTH_TIMEOUT_MS = 5000

/**
 * The full URL for an API path. VITE_API_URL is the API's origin when it runs
 * elsewhere (locally http://localhost:8080). It is empty in production, where
 * Caddy serves the app and the API from one domain and paths stay relative.
 */
export function apiUrl(path: string): string {
  const base = (import.meta.env.VITE_API_URL ?? '').replace(/\/+$/, '')
  return `${base}${path}`
}

/**
 * True only when GET /healthz answers 200 {"status":"ok"}. Any other status,
 * an unexpected body, a network error, or an abort counts as unhealthy, so
 * this never throws.
 */
export async function isApiHealthy(signal?: AbortSignal): Promise<boolean> {
  try {
    // credentials: the API's cookie-based sessions need it from the next phase on;
    // using it here proves the CORS setup works with credentials.
    const res = await fetch(apiUrl('/healthz'), { credentials: 'include', signal })
    if (res.status !== 200) {
      return false
    }
    const body: unknown = await res.json()
    return typeof body === 'object' && body !== null && 'status' in body && body.status === 'ok'
  } catch {
    return false
  }
}

/** An API call that failed. `code` and `message` come from the API's error shape when it sent one. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

type ApiFetchOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  /** Sent as JSON. */
  body?: unknown
  signal?: AbortSignal
}

/**
 * Calls the API with the session cookie and returns the JSON answer (undefined
 * for 204). Every failure becomes an ApiError with a message that can be shown
 * as is, except an abort, which is rethrown unchanged so callers can ignore it.
 */
export async function apiFetch<T = unknown>(path: string, { method = 'GET', body, signal }: ApiFetchOptions = {}): Promise<T> {
  const headers = new Headers({ Accept: 'application/json' })
  // The API refuses writes that aren't marked as JSON (415), body or not.
  if (method === 'POST' || method === 'PUT' || method === 'PATCH') {
    headers.set('Content-Type', 'application/json')
  }

  let res: Response
  try {
    res = await fetch(apiUrl(path), {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'include',
      signal,
    })
  } catch (err) {
    if (isAbort(err)) {
      throw err
    }
    throw new ApiError(0, 'network_error', "Can't reach the server. Check your connection and try again.")
  }

  if (!res.ok) {
    throw await errorFrom(res)
  }
  if (res.status === 204) {
    return undefined as T
  }
  try {
    return (await res.json()) as T
  } catch {
    throw new ApiError(res.status, 'bad_response', 'The server sent an answer we could not read. Please try again.')
  }
}

async function errorFrom(res: Response): Promise<ApiError> {
  try {
    const body: unknown = await res.json()
    if (isErrorShape(body)) {
      return new ApiError(res.status, body.error.code, body.error.message)
    }
  } catch {
    // Not JSON, e.g. a proxy's HTML error page.
  }
  return new ApiError(res.status, 'http_error', `Something went wrong (HTTP ${res.status}). Please try again.`)
}

function isErrorShape(v: unknown): v is { error: { code: string; message: string } } {
  if (typeof v !== 'object' || v === null || !('error' in v)) {
    return false
  }
  const e = v.error
  return typeof e === 'object' && e !== null && 'code' in e && typeof e.code === 'string' && 'message' in e && typeof e.message === 'string'
}

function isAbort(err: unknown): boolean {
  return typeof err === 'object' && err !== null && 'name' in err && err.name === 'AbortError'
}
