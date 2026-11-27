import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { routes } from '../routes'
import { apiErrorResponse, jsonResponse, mockApi } from '../test/mockApi'

const seller = { id: 7, email: 'rafi@example.com', shopName: 'Toko Rafi' }
const loggedOut = () => apiErrorResponse(401, 'unauthenticated')

function renderSignup() {
  const router = createMemoryRouter(routes, { initialEntries: ['/signup'] })
  render(<RouterProvider router={router} />)
  return router
}

function fillAndSubmit(values: { email?: string; password?: string; shopName?: string }) {
  if (values.email !== undefined) fireEvent.change(screen.getByLabelText('Email'), { target: { value: values.email } })
  if (values.password !== undefined) fireEvent.change(screen.getByLabelText('Password'), { target: { value: values.password } })
  if (values.shopName !== undefined) fireEvent.change(screen.getByLabelText('Shop name'), { target: { value: values.shopName } })
  fireEvent.click(screen.getByRole('button', { name: 'Create account' }))
}

describe('sign up page', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', '')
  })

  it('creates the account, logs in, and goes to the products page', async () => {
    const fetchMock = mockApi({ '/api/me': loggedOut, 'POST /api/auth/signup': () => jsonResponse(201, seller) })
    const router = renderSignup()
    await screen.findByRole('heading', { name: 'Sign up' })

    fillAndSubmit({ email: 'rafi@example.com', password: 'correct horse', shopName: 'Toko Rafi' })

    expect(await screen.findByRole('heading', { name: 'Products' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/products')
    expect(within(screen.getByRole('banner')).getByText('Toko Rafi')).toBeInTheDocument()

    const signupCall = fetchMock.mock.calls.find(([url]) => typeof url === 'string' && url.endsWith('/api/auth/signup'))
    const body = signupCall?.[1]?.body
    expect(signupCall?.[1]?.method).toBe('POST')
    expect(typeof body === 'string' ? JSON.parse(body) : body).toEqual({
      email: 'rafi@example.com',
      password: 'correct horse',
      shopName: 'Toko Rafi',
    })
  })

  it("shows each field's error next to it", async () => {
    mockApi({
      '/api/me': loggedOut,
      'POST /api/auth/signup': () =>
        jsonResponse(400, {
          error: {
            code: 'validation_failed',
            message: 'Some fields need fixing.',
            fields: { password: 'Use at least 8 characters.', shopName: "Enter your shop's name." },
          },
        }),
    })
    renderSignup()
    await screen.findByRole('heading', { name: 'Sign up' })

    fillAndSubmit({ email: 'rafi@example.com', password: 'short', shopName: '' })

    const password = screen.getByLabelText('Password')
    expect(await screen.findByText('Use at least 8 characters.')).toBeInTheDocument()
    expect(password).toHaveAttribute('aria-invalid', 'true')
    expect(password).toHaveAccessibleDescription('Use at least 8 characters.')
    expect(screen.getByLabelText('Shop name')).toHaveAccessibleDescription("Enter your shop's name.")
    expect(screen.getByLabelText('Email')).not.toHaveAttribute('aria-invalid', 'true')
  })

  it('shows "email taken" on the email field, with a link to log in', async () => {
    mockApi({
      '/api/me': loggedOut,
      'POST /api/auth/signup': () => apiErrorResponse(409, 'email_taken', 'That email already has an account. Log in instead?'),
    })
    renderSignup()
    await screen.findByRole('heading', { name: 'Sign up' })

    fillAndSubmit({ email: 'taken@example.com', password: 'correct horse', shopName: 'Toko' })

    expect(await screen.findByText('That email already has an account. Log in instead?')).toBeInTheDocument()
    expect(screen.getByLabelText('Email')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('link', { name: 'Log in with that email' })).toHaveAttribute('href', '/login')
  })

  it('shows other failures as an alert and keeps what was typed', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((input: string) =>
        input.endsWith('/api/me') ? Promise.resolve(loggedOut()) : Promise.reject(new TypeError('Failed to fetch')),
      ),
    )
    renderSignup()
    await screen.findByRole('heading', { name: 'Sign up' })

    fillAndSubmit({ email: 'rafi@example.com', password: 'correct horse', shopName: 'Toko Rafi' })

    expect(await screen.findByRole('alert')).toHaveTextContent("Can't reach the server")
    expect(screen.getByLabelText('Email')).toHaveValue('rafi@example.com')
  })

  it('disables the button while the account is being created', async () => {
    mockApi({ '/api/me': loggedOut, 'POST /api/auth/signup': () => new Promise<Response>(() => {}) })
    renderSignup()
    await screen.findByRole('heading', { name: 'Sign up' })

    fillAndSubmit({ email: 'rafi@example.com', password: 'correct horse', shopName: 'Toko Rafi' })

    expect(await screen.findByRole('button', { name: /creating/i })).toBeDisabled()
  })

  it('sends a seller who is already logged in to the products page', async () => {
    mockApi({ '/api/me': () => jsonResponse(200, seller) })

    const router = renderSignup()

    expect(await screen.findByRole('heading', { name: 'Products' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/products')
  })
})
