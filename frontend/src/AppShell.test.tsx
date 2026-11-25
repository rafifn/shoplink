import { render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { routes } from './routes'
import { apiErrorResponse, jsonResponse, mockApi } from './test/mockApi'

const seller = { id: 1, email: 'rafi@example.com', shopName: 'Toko Rafi' }
const healthy = () => jsonResponse(200, { status: 'ok' })

function renderAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  render(<RouterProvider router={router} />)
  return router
}

describe('app shell', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', '')
  })

  it('shows log in and sign up links when logged out', async () => {
    mockApi({ '/healthz': healthy, '/api/me': () => apiErrorResponse(401, 'unauthenticated') })

    renderAt('/')

    const header = screen.getByRole('banner')
    expect(await within(header).findByRole('link', { name: 'Log in' })).toHaveAttribute('href', '/login')
    expect(within(header).getByRole('link', { name: 'Sign up' })).toHaveAttribute('href', '/signup')
    expect(within(header).queryByText(seller.shopName)).not.toBeInTheDocument()
  })

  it('shows the shop name and a products link when logged in', async () => {
    mockApi({ '/healthz': healthy, '/api/me': () => jsonResponse(200, seller) })

    renderAt('/')

    const header = screen.getByRole('banner')
    expect(await within(header).findByText(seller.shopName)).toBeInTheDocument()
    expect(within(header).getByRole('link', { name: 'Products' })).toHaveAttribute('href', '/products')
    expect(within(header).queryByRole('link', { name: 'Log in' })).not.toBeInTheDocument()
  })

  it('keeps the home page API status', async () => {
    mockApi({ '/healthz': healthy, '/api/me': () => apiErrorResponse(401, 'unauthenticated') })

    renderAt('/')

    expect(await screen.findByText('API OK')).toBeInTheDocument()
  })
})

describe('seller-only pages', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', '')
  })

  it('send logged-out visitors to /login and remember where they were going', async () => {
    mockApi({ '/api/me': () => apiErrorResponse(401, 'unauthenticated') })

    const router = renderAt('/products?page=2')

    expect(await screen.findByRole('heading', { name: 'Log in' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
    expect(new URLSearchParams(router.state.location.search).get('next')).toBe('/products?page=2')
  })

  it('show the page to a logged-in seller', async () => {
    mockApi({ '/api/me': () => jsonResponse(200, seller) })

    const router = renderAt('/products')

    expect(await screen.findByRole('heading', { name: 'Products' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/products')
  })

  it('show a loading message while the login check runs', () => {
    mockApi({ '/api/me': () => new Promise<Response>(() => {}) }) // never answers

    renderAt('/products')

    expect(screen.getByText(/loading/i)).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Products' })).not.toBeInTheDocument()
  })

  it('show the error instead of redirecting when the login check fails', async () => {
    mockApi({ '/api/me': () => apiErrorResponse(500, 'internal_error', 'Something went wrong on our side.') })

    const router = renderAt('/products')

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong on our side.')
    expect(router.state.location.pathname).toBe('/products')
  })

  it('treat an unexpected /api/me body as an error, not as logged in', async () => {
    mockApi({ '/api/me': () => jsonResponse(200, { status: 'ok' }) })

    renderAt('/products')

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Products' })).not.toBeInTheDocument()
  })
})
