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
