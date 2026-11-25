import { vi } from 'vitest'

export function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/** A response in the API's error shape: {"error": {"code", "message"}}. */
export function apiErrorResponse(status: number, code: string, message = `${code} from the API`): Response {
  return jsonResponse(status, { error: { code, message } })
}

type Handler = () => Response | Promise<Response>

/**
 * Replaces fetch with a fake API. Keys are "METHOD /path" or just "/path" (any
 * method); unknown paths get the API's JSON 404. Returns the mock to inspect calls.
 */
export function mockApi(routes: Record<string, Handler>) {
  const fetchMock = vi.fn((input: string | URL | Request, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const path = new URL(url, 'http://localhost').pathname
    const handler = routes[`${init?.method ?? 'GET'} ${path}`] ?? routes[path]
    return Promise.resolve(handler ? handler() : apiErrorResponse(404, 'not_found'))
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}
