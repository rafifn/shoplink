import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { formatRupiah } from '../money'
import { routes } from '../routes'
import { apiErrorResponse, jsonResponse, mockApi } from '../test/mockApi'

const me = () => jsonResponse(200, { id: 1, email: 'rafi@example.com', shopName: 'Toko Rafi' })
const mug = { id: 5, name: 'Stoneware mug', description: '350 ml', priceSen: 100_075, stock: 12, imageUrl: 'https://img.example.com/mug.jpg' }

function renderAt(path: string) {
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  render(<RouterProvider router={router} />)
  return router
}

function patchCalls(fetchMock: ReturnType<typeof mockApi>) {
  return fetchMock.mock.calls.filter(([url, init]) => typeof url === 'string' && url.endsWith('/api/products/5') && init?.method === 'PATCH')
}

describe('edit product page', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', '')
  })

  it("is filled in with the product's current details", async () => {
    mockApi({ '/api/me': me, 'GET /api/products/5': () => jsonResponse(200, mug) })
    renderAt('/products/5/edit')

    expect(await screen.findByRole('heading', { name: 'Edit Stoneware mug' })).toBeInTheDocument()
    expect(screen.getByLabelText('Name')).toHaveValue('Stoneware mug')
    expect(screen.getByLabelText('Description')).toHaveValue('350 ml')
    expect(screen.getByLabelText('Price')).toHaveValue(formatRupiah(100_075))
    expect(screen.getByLabelText('Stock')).toHaveValue('12')
    expect(screen.getByLabelText('Image URL')).toHaveValue('https://img.example.com/mug.jpg')
  })

  it('saves the changes and returns to the list with a message', async () => {
    const saved = { ...mug, name: 'Big mug', priceSen: 9_000_000, imageUrl: null }
    const fetchMock = mockApi({
      '/api/me': me,
      'GET /api/products/5': () => jsonResponse(200, mug),
      'PATCH /api/products/5': () => jsonResponse(200, saved),
      'GET /api/products': () => jsonResponse(200, { products: [saved] }),
    })
    const router = renderAt('/products/5/edit')
    await screen.findByRole('heading', { name: 'Edit Stoneware mug' })

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'Big mug' } })
    fireEvent.change(screen.getByLabelText('Price'), { target: { value: '90.000' } })
    fireEvent.change(screen.getByLabelText('Image URL'), { target: { value: '' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Saved Big mug.')
    expect(router.state.location.pathname).toBe('/products')
    const body = patchCalls(fetchMock)[0]?.[1]?.body
    expect(typeof body === 'string' ? JSON.parse(body) : body).toEqual({
      name: 'Big mug',
      description: '350 ml',
      priceSen: 9_000_000,
      stock: 12,
      imageUrl: null,
    })
  })

  it("says the product doesn't exist for a 404 (another seller's, archived, or missing)", async () => {
    mockApi({ '/api/me': me, 'GET /api/products/5': () => apiErrorResponse(404, 'not_found', "That product doesn't exist.") })
    renderAt('/products/5/edit')

    expect(await screen.findByRole('heading', { name: 'Product not found' })).toBeInTheDocument()
    expect(within(screen.getByRole('main')).getByRole('link', { name: /back to your products/i })).toHaveAttribute('href', '/products')
    expect(screen.queryByLabelText('Name')).not.toBeInTheDocument()
  })

  it('shows other loading errors as an alert', async () => {
    mockApi({ '/api/me': me, 'GET /api/products/5': () => apiErrorResponse(500, 'internal_error', 'Something went wrong on our side.') })
    renderAt('/products/5/edit')

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong on our side.')
  })

  it('catches a bad price before sending', async () => {
    const fetchMock = mockApi({ '/api/me': me, 'GET /api/products/5': () => jsonResponse(200, mug) })
    renderAt('/products/5/edit')
    await screen.findByRole('heading', { name: 'Edit Stoneware mug' })

    fireEvent.change(screen.getByLabelText('Price'), { target: { value: '1.5' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(await screen.findByText(/enter a price like 85\.000/i)).toBeInTheDocument()
    expect(patchCalls(fetchMock)).toHaveLength(0)
  })

  it("shows the API's field messages and stays on the page", async () => {
    mockApi({
      '/api/me': me,
      'GET /api/products/5': () => jsonResponse(200, mug),
      'PATCH /api/products/5': () =>
        jsonResponse(400, { error: { code: 'validation_failed', message: 'Some fields need fixing.', fields: { name: 'Enter a product name.' } } }),
    })
    const router = renderAt('/products/5/edit')
    await screen.findByRole('heading', { name: 'Edit Stoneware mug' })

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: ' ' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(await screen.findByText('Enter a product name.')).toBeInTheDocument()
    expect(screen.getByLabelText('Name')).toHaveAccessibleDescription('Enter a product name.')
    expect(router.state.location.pathname).toBe('/products/5/edit')
  })

  it('treats a non-numeric id as not found without asking the API', async () => {
    const fetchMock = mockApi({ '/api/me': me })
    renderAt('/products/abc/edit')

    expect(await screen.findByRole('heading', { name: 'Product not found' })).toBeInTheDocument()
    expect(fetchMock.mock.calls.some(([url]) => typeof url === 'string' && url.includes('/api/products/'))).toBe(false)
  })
})

describe('products list', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', '')
  })

  it('links each product to its edit page', async () => {
    mockApi({ '/api/me': me, 'GET /api/products': () => jsonResponse(200, { products: [mug] }) })
    renderAt('/products')

    expect(await screen.findByRole('link', { name: 'Edit Stoneware mug' })).toHaveAttribute('href', '/products/5/edit')
  })
})
