import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter } from 'react-router'
import { RouterProvider } from 'react-router/dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { routes } from '../routes'
import { apiErrorResponse, jsonResponse, mockApi } from '../test/mockApi'

const seller = { id: 1, email: 'rafi@example.com', shopName: 'Toko Rafi' }
const me = () => jsonResponse(200, seller)
// toHaveTextContent collapses whitespace, so Intl's non-breaking space after "Rp" reads as a
// plain space here (money.test.ts checks the exact character).

const candle = { id: 2, name: 'Soy candle', description: 'Sandalwood', priceSen: 8_500_000, stock: 0, imageUrl: null }
const mug = { id: 1, name: 'Stoneware mug', description: '', priceSen: 100_075, stock: 12, imageUrl: 'https://img.example.com/mug.jpg' }

function renderProducts() {
  render(<RouterProvider router={createMemoryRouter(routes, { initialEntries: ['/products'] })} />)
}

async function productItems() {
  return within(await screen.findByRole('list', { name: 'Your products' })).getAllByRole('listitem')
}

function fill(values: Record<string, string>) {
  for (const [label, value] of Object.entries(values)) {
    fireEvent.change(screen.getByLabelText(label), { target: { value } })
  }
}

function postCalls(fetchMock: ReturnType<typeof mockApi>) {
  return fetchMock.mock.calls.filter(([url, init]) => typeof url === 'string' && url.endsWith('/api/products') && init?.method === 'POST')
}

describe('products page', () => {
  beforeEach(() => {
    vi.stubEnv('VITE_API_URL', '')
  })

  it("lists the seller's products in the order the API gives (newest first)", async () => {
    mockApi({ '/api/me': me, 'GET /api/products': () => jsonResponse(200, { products: [candle, mug] }) })
    renderProducts()

    const [first, second] = await productItems()
    expect(first).toHaveTextContent('Soy candle')
    expect(first).toHaveTextContent('Rp 85.000')
    expect(first).toHaveTextContent('Out of stock')
    expect(second).toHaveTextContent('Stoneware mug')
    expect(second).toHaveTextContent('Rp 1.000,75')
    expect(second).toHaveTextContent('12 in stock')
    expect(within(second).getByRole('img', { name: 'Stoneware mug' })).toHaveAttribute('src', 'https://img.example.com/mug.jpg')
    expect(within(first).queryByRole('img')).not.toBeInTheDocument()
  })

  it('says so when there are no products yet', async () => {
    mockApi({ '/api/me': me, 'GET /api/products': () => jsonResponse(200, { products: [] }) })
    renderProducts()

    expect(await screen.findByText(/no products yet/i)).toBeInTheDocument()
  })

  it('shows an error when the products cannot be loaded', async () => {
    mockApi({ '/api/me': me, 'GET /api/products': () => apiErrorResponse(500, 'internal_error', 'Something went wrong on our side.') })
    renderProducts()

    expect(await screen.findByRole('alert')).toHaveTextContent('Something went wrong on our side.')
  })

  it('adds a product, shows it first, and clears the form', async () => {
    const created = { id: 3, name: 'Linen tote', description: 'Natural linen', priceSen: 100_075, stock: 3, imageUrl: null }
    const fetchMock = mockApi({
      '/api/me': me,
      'GET /api/products': () => jsonResponse(200, { products: [mug] }),
      'POST /api/products': () => jsonResponse(201, created),
    })
    renderProducts()
    await productItems()

    fill({ Name: 'Linen tote', Description: 'Natural linen', Price: 'Rp 1.000,75', Stock: '3', 'Image URL': '' })
    fireEvent.click(screen.getByRole('button', { name: 'Add product' }))

    expect(await screen.findByRole('status')).toHaveTextContent('Added Linen tote.')
    const items = await productItems()
    expect(items).toHaveLength(2)
    expect(items[0]).toHaveTextContent('Linen tote')
    expect(screen.getByLabelText('Name')).toHaveValue('')
    expect(screen.getByLabelText('Price')).toHaveValue('')

    const body = postCalls(fetchMock)[0]?.[1]?.body
    expect(typeof body === 'string' ? JSON.parse(body) : body).toEqual({
      name: 'Linen tote',
      description: 'Natural linen',
      priceSen: 100_075,
      stock: 3,
      imageUrl: null,
    })
  })

  it.each([
    ['a decimal point', '1.5'],
    ['three decimals', '1.000,755'],
    ['nothing', ''],
  ])('catches a price with %s before sending it', async (_name, price) => {
    const fetchMock = mockApi({ '/api/me': me, 'GET /api/products': () => jsonResponse(200, { products: [] }) })
    renderProducts()
    await screen.findByText(/no products yet/i)

    fill({ Name: 'Mug', Price: price, Stock: '1' })
    fireEvent.click(screen.getByRole('button', { name: 'Add product' }))

    expect(await screen.findByText(/enter a price like 85\.000/i)).toBeInTheDocument()
    expect(screen.getByLabelText('Price')).toHaveAttribute('aria-invalid', 'true')
    expect(postCalls(fetchMock)).toHaveLength(0)
  })

  it.each([['two'], ['1,5'], ['-1'], ['']])('catches stock %j before sending it', async (stock) => {
    const fetchMock = mockApi({ '/api/me': me, 'GET /api/products': () => jsonResponse(200, { products: [] }) })
    renderProducts()
    await screen.findByText(/no products yet/i)

    fill({ Name: 'Mug', Price: '85.000', Stock: stock })
    fireEvent.click(screen.getByRole('button', { name: 'Add product' }))

    expect(await screen.findByText(/whole number/i)).toBeInTheDocument()
    expect(postCalls(fetchMock)).toHaveLength(0)
  })

  it("shows the API's field messages next to the right fields", async () => {
    mockApi({
      '/api/me': me,
      'GET /api/products': () => jsonResponse(200, { products: [] }),
      'POST /api/products': () =>
        jsonResponse(400, {
          error: {
            code: 'validation_failed',
            message: 'Some fields need fixing.',
            fields: { name: 'Enter a product name.', priceSen: 'That price is too high.', imageUrl: 'Enter an image URL that starts with http:// or https://.' },
          },
        }),
    })
    renderProducts()
    await screen.findByText(/no products yet/i)

    fill({ Name: ' ', Price: '85.000', Stock: '1', 'Image URL': 'javascript:alert(1)' })
    fireEvent.click(screen.getByRole('button', { name: 'Add product' }))

    expect(await screen.findByText('That price is too high.')).toBeInTheDocument()
    expect(screen.getByLabelText('Price')).toHaveAccessibleDescription('That price is too high.')
    expect(screen.getByLabelText('Name')).toHaveAccessibleDescription('Enter a product name.')
    expect(screen.getByLabelText('Image URL')).toHaveAttribute('aria-invalid', 'true')
  })
})
