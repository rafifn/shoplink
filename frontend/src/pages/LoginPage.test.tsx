import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { routes } from '../routes'
import { apiErrorResponse, jsonResponse, mockApi } from '../test/mockApi'

const seller = { id: 7, email: 'rafi@example.com', shopName: 'Toko Rafi' }
const loggedOut = () => apiErrorResponse(401, 'unauthenticated')

function renderAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  render(<RouterProvider router={router} />)
  return router
}

async function logIn(email = 'rafi@example.com', password = 'correct horse') {
  await screen.findByRole('heading', { name: 'Log in' })
  fireEvent.change(screen.getByLabelText('Email'), { target: { value: email } })
  fireEvent.change(screen.getByLabelText('Password'), { target: { value: password } })
  fireEvent.click(within(screen.getByRole('main')).getByRole('button', { name: 'Log in' }))
}

describe('log in page', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', '')
  })

  it('logs in and goes to the products page', async () => {
    const fetchMock = mockApi({ '/api/me': loggedOut, 'POST /api/auth/login': () => jsonResponse(200, seller) })
    const router = renderAt('/login')

    await logIn()

    expect(await screen.findByRole('heading', { name: 'Products' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/products')
    expect(within(screen.getByRole('banner')).getByText('Toko Rafi')).toBeInTheDocument()
    const call = fetchMock.mock.calls.find(([url]) => typeof url === 'string' && url.endsWith('/api/auth/login'))
    const body = call?.[1]?.body
    expect(typeof body === 'string' ? JSON.parse(body) : body).toEqual({ email: 'rafi@example.com', password: 'correct horse' })
  })

  it('returns to the page in ?next', async () => {
    mockApi({ '/api/me': loggedOut, 'POST /api/auth/login': () => jsonResponse(200, seller) })
    const router = renderAt(`/login?next=${encodeURIComponent('/products?page=2')}`)

    await logIn()

    await screen.findByRole('heading', { name: 'Products' })
    expect(router.state.location.pathname + router.state.location.search).toBe('/products?page=2')
  })

  it.each([
    ['another site', 'https://evil.example.com/'],
    ['a protocol-relative URL', '//evil.example.com'],
    ['a backslash trick', '/\\evil.example.com'],
    ['a relative path', 'products'],
  ])('ignores ?next pointing to %s', async (_name, next) => {
    mockApi({ '/api/me': loggedOut, 'POST /api/auth/login': () => jsonResponse(200, seller) })
    const router = renderAt(`/login?next=${encodeURIComponent(next)}`)

    await logIn()

    await screen.findByRole('heading', { name: 'Products' })
    expect(router.state.location.pathname).toBe('/products')
  })

  it('shows the error for a wrong email or password and keeps the email', async () => {
    mockApi({
      '/api/me': loggedOut,
      'POST /api/auth/login': () => apiErrorResponse(401, 'invalid_credentials', "That email and password don't match an account."),
    })
    const router = renderAt('/login')

    await logIn('rafi@example.com', 'wrong')

    expect(await screen.findByRole('alert')).toHaveTextContent("That email and password don't match an account.")
    expect(screen.getByLabelText('Email')).toHaveValue('rafi@example.com')
    expect(router.state.location.pathname).toBe('/login')
  })

  it('shows missing-field messages next to the fields', async () => {
    mockApi({
      '/api/me': loggedOut,
      'POST /api/auth/login': () =>
        jsonResponse(400, {
          error: { code: 'validation_failed', message: 'Some fields need fixing.', fields: { password: 'Enter your password.' } },
        }),
    })
    renderAt('/login')

    await logIn('rafi@example.com', '')

    expect(await screen.findByText('Enter your password.')).toBeInTheDocument()
    expect(screen.getByLabelText('Password')).toHaveAccessibleDescription('Enter your password.')
  })

  it('sends a seller who is already logged in on to ?next', async () => {
    mockApi({ '/api/me': () => jsonResponse(200, seller) })

    const router = renderAt(`/login?next=${encodeURIComponent('/products?page=3')}`)

    await screen.findByRole('heading', { name: 'Products' })
    expect(router.state.location.search).toBe('?page=3')
  })
})

describe('log out button', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', '')
  })

  it('logs out and goes home', async () => {
    const fetchMock = mockApi({
      '/api/me': () => jsonResponse(200, seller),
      '/healthz': () => jsonResponse(200, { status: 'ok' }),
      'POST /api/auth/logout': () => new Response(null, { status: 204 }),
    })
    const router = renderAt('/products')
    const header = screen.getByRole('banner')

    fireEvent.click(await within(header).findByRole('button', { name: 'Log out' }))

    expect(await within(header).findByRole('link', { name: 'Log in' })).toBeInTheDocument()
    expect(within(header).queryByText('Toko Rafi')).not.toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/')
    expect(fetchMock.mock.calls.some(([url, init]) => typeof url === 'string' && url.endsWith('/api/auth/logout') && init?.method === 'POST')).toBe(true)
  })

  it('stays logged in and says so when logging out fails', async () => {
    mockApi({
      '/api/me': () => jsonResponse(200, seller),
      'POST /api/auth/logout': () => apiErrorResponse(500, 'internal_error', 'Something went wrong on our side.'),
    })
    const router = renderAt('/products')
    const header = screen.getByRole('banner')

    fireEvent.click(await within(header).findByRole('button', { name: 'Log out' }))

    expect(await within(header).findByRole('alert')).toHaveTextContent(/couldn't log out/i)
    expect(within(header).getByText('Toko Rafi')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/products')
  })
})
