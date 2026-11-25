import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, apiFetch, apiUrl } from './api'
import { apiErrorResponse, jsonResponse, mockApi } from './test/mockApi'

describe('apiUrl', () => {
  it.each([
    ['http://localhost:8080', 'http://localhost:8080/healthz'],
    ['http://localhost:8080/', 'http://localhost:8080/healthz'],
    // Production: the app and the API share a domain, so requests stay same-origin.
    ['', '/healthz'],
  ])('VITE_API_URL=%j gives %s', (base, want) => {
    vi.stubEnv('VITE_API_URL', base)
    expect(apiUrl('/healthz')).toBe(want)
  })
})

describe('apiFetch', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', 'http://api.test')
  })

  it('sends cookies and returns the JSON body', async () => {
    const fetchMock = mockApi({ 'GET /api/me': () => jsonResponse(200, { id: 1 }) })

    await expect(apiFetch('/api/me')).resolves.toEqual({ id: 1 })

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('http://api.test/api/me')
    expect(init?.credentials).toBe('include')
    expect(new Headers(init?.headers).has('Content-Type')).toBe(false)
  })

  it('sends a body as JSON', async () => {
    const fetchMock = mockApi({ 'POST /api/things': () => jsonResponse(201, { id: 7 }) })

    await expect(apiFetch('/api/things', { method: 'POST', body: { name: 'Mug' } })).resolves.toEqual({ id: 7 })

    const init = fetchMock.mock.calls[0][1]
    expect(new Headers(init?.headers).get('Content-Type')).toBe('application/json')
    expect(init?.body).toBe('{"name":"Mug"}')
  })

  it('marks writes as JSON even without a body (the API answers 415 otherwise)', async () => {
    const fetchMock = mockApi({ 'POST /api/auth/logout': () => new Response(null, { status: 204 }) })

    await expect(apiFetch('/api/auth/logout', { method: 'POST' })).resolves.toBeUndefined()

    expect(new Headers(fetchMock.mock.calls[0][1]?.headers).get('Content-Type')).toBe('application/json')
  })

  it('throws ApiError with the code and message from the API', async () => {
    mockApi({ '/api/auth/signup': () => apiErrorResponse(409, 'email_taken', 'That email already has an account.') })

    const err = await apiFetch('/api/auth/signup', { method: 'POST', body: {} }).catch((e: unknown) => e)

    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 409, code: 'email_taken', message: 'That email already has an account.' })
  })

  it('throws a readable ApiError for an error that is not JSON', async () => {
    mockApi({ '/api/me': () => new Response('<h1>502 Bad Gateway</h1>', { status: 502 }) })

    const err = await apiFetch('/api/me').catch((e: unknown) => e)

    expect(err).toMatchObject({ status: 502, code: 'http_error' })
    expect((err as Error).message).toMatch(/something went wrong/i)
  })

  it('throws a readable ApiError when the server cannot be reached', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))

    const err = await apiFetch('/api/me').catch((e: unknown) => e)

    expect(err).toMatchObject({ status: 0, code: 'network_error' })
    expect((err as Error).message).toMatch(/can't reach the server/i)
  })

  it('lets an abort through unchanged, so callers can ignore it', async () => {
    const abort = new DOMException('The operation was aborted.', 'AbortError')
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(abort))

    await expect(apiFetch('/api/me')).rejects.toBe(abort)
  })
})
