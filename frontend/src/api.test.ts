import { describe, expect, it, vi } from 'vitest'

import { apiUrl } from './api'

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
