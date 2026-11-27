import { formatRupiah, parseRupiah } from '../money'

/** A product as the API returns it. */
export type Product = {
  id: number
  name: string
  description: string
  priceSen: number
  stock: number
  imageUrl: string | null
}

/** What the API takes to create or replace a product. */
export type ProductBody = Omit<Product, 'id'>

/** The product form's inputs, as typed. */
export type ProductFormValues = { name: string; description: string; price: string; stock: string; imageUrl: string }

export const emptyProductForm: ProductFormValues = { name: '', description: '', price: '', stock: '', imageUrl: '' }

/** Form values for editing an existing product. */
export function formValuesFrom(p: Product): ProductFormValues {
  return {
    name: p.name,
    description: p.description,
    price: formatRupiah(p.priceSen),
    stock: String(p.stock),
    imageUrl: p.imageUrl ?? '',
  }
}

/**
 * Turns the typed form into a request body, or into messages keyed by the
 * API's field names (priceSen, stock) when the price or stock can't be read.
 * Nothing is sent while either is unclear.
 */
export function toProductBody(v: ProductFormValues): { body: ProductBody } | { errors: Record<string, string> } {
  const priceSen = parseRupiah(v.price)
  const stockText = v.stock.trim()
  const stock = /^\d+$/.test(stockText) ? Number(stockText) : null

  const errors: Record<string, string> = {}
  if (priceSen === null) {
    errors.priceSen = 'Enter a price like 85.000 or 1.000,75.'
  }
  if (stock === null) {
    errors.stock = 'Enter stock as a whole number, like 12.'
  }
  if (priceSen === null || stock === null) {
    return { errors }
  }
  const imageUrl = v.imageUrl.trim()
  return {
    body: { name: v.name, description: v.description, priceSen, stock, imageUrl: imageUrl === '' ? null : imageUrl },
  }
}

export function isProduct(v: unknown): v is Product {
  if (typeof v !== 'object' || v === null) {
    return false
  }
  const p = v as Record<string, unknown>
  return (
    typeof p.id === 'number' &&
    typeof p.name === 'string' &&
    typeof p.description === 'string' &&
    typeof p.priceSen === 'number' &&
    typeof p.stock === 'number' &&
    (p.imageUrl === null || typeof p.imageUrl === 'string')
  )
}

export function isProductList(v: unknown): v is { products: Product[] } {
  return typeof v === 'object' && v !== null && 'products' in v && Array.isArray(v.products) && v.products.every(isProduct)
}
