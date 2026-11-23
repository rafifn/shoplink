import { act, render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { HEALTH_TIMEOUT_MS } from '../api'
import { routes } from '../routes'

function renderAt(path: string) {
  render(<RouterProvider router={createMemoryRouter(routes, { initialEntries: [path] })} />)
}

function jsonResponse(status: number, body: unknown) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('home page', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', 'http://api.test')
  })

  it('shows "API OK" when /healthz answers 200 {"status":"ok"}', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { status: 'ok' }))
    vi.stubGlobal('fetch', fetchMock)

    renderAt('/')

    expect(screen.getByRole('status')).toHaveTextContent('Checking API')
    expect(await screen.findByText('API OK')).toBeInTheDocument()
    expect(fetchMock).toHaveBeenCalledWith(
      'http://api.test/healthz',
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it.each([
    ['503 with the API error shape', () => jsonResponse(503, { error: { code: 'db_unavailable', message: 'down' } })],
    ['500 with an HTML body', () => new Response('<h1>Bad gateway</h1>', { status: 500 })],
    ['200 with a different status', () => jsonResponse(200, { status: 'starting' })],
    ['200 with a body that is not JSON', () => new Response('ok', { status: 200 })],
  ])('shows "API unreachable" on %s', async (_name, response) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response()))

    renderAt('/')

    expect(await screen.findByText('API unreachable')).toBeInTheDocument()
  })

  it('shows "API unreachable" when the request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))

    renderAt('/')

    expect(await screen.findByText('API unreachable')).toBeInTheDocument()
  })

  describe('with a hanging API', () => {
    beforeEach(() => {
      vi.useFakeTimers()
    })
    afterEach(() => {
      vi.useRealTimers()
    })

    it('shows "API unreachable" once the request times out', async () => {
      // Never answers; rejects only when the page aborts the request.
      const hanging = vi.fn(
        (_url: string, init?: RequestInit) =>
          new Promise<Response>((_resolve, reject) => {
            init?.signal?.addEventListener('abort', () => {
              reject(new DOMException('The operation was aborted.', 'AbortError'))
            })
          }),
      )
      vi.stubGlobal('fetch', hanging)

      renderAt('/')
      await act(() => vi.advanceTimersByTimeAsync(HEALTH_TIMEOUT_MS - 1))
      expect(screen.getByRole('status')).toHaveTextContent('Checking API')

      await act(() => vi.advanceTimersByTimeAsync(1))
      expect(screen.getByRole('status')).toHaveTextContent('API unreachable')
    })
  })
})

describe('routing', () => {
  it('shows a not-found page for unknown paths', () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { status: 'ok' })))

    renderAt('/no-such-page')

    expect(screen.getByRole('heading', { name: 'Page not found' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /home/i })).toHaveAttribute('href', '/')
  })
})
