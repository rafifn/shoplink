import { type FormEvent, useEffect, useState } from 'react'

import { ApiError, apiFetch } from '../api'
import { Field, TextAreaField } from '../components/Field'
import { formatRupiah, parseRupiah } from '../money'

/** A product as the API returns it. */
type Product = {
  id: number
  name: string
  description: string
  priceSen: number
  stock: number
  imageUrl: string | null
}

type ListState = { status: 'loading' } | { status: 'error'; message: string } | { status: 'ready'; products: Product[] }

type FormValues = { name: string; description: string; price: string; stock: string; imageUrl: string }
const emptyForm: FormValues = { name: '', description: '', price: '', stock: '', imageUrl: '' }

/** The seller's products, newest first, with a form to add one. */
export function ProductsPage() {
  const [list, setList] = useState<ListState>({ status: 'loading' })

  useEffect(() => {
    const controller = new AbortController()
    apiFetch('/api/products', { signal: controller.signal })
      .then((body) => {
        if (controller.signal.aborted) {
          return
        }
        setList(
          isProductList(body)
            ? { status: 'ready', products: body.products }
            : { status: 'error', message: 'The server sent an unexpected answer. Please reload the page.' },
        )
      })
      .catch((err: unknown) => {
        if (!controller.signal.aborted) {
          setList({ status: 'error', message: err instanceof ApiError ? err.message : 'Could not load your products.' })
        }
      })
    return () => {
      controller.abort()
    }
  }, [])

  return (
    <section>
      <h1>Products</h1>

      {list.status === 'loading' && <p>Loading your products…</p>}
      {list.status === 'error' && <p role="alert">{list.message}</p>}
      {list.status === 'ready' &&
        (list.products.length === 0 ? (
          <p>No products yet. Add your first one below.</p>
        ) : (
          <ul className="product-list" aria-label="Your products">
            {list.products.map((p) => (
              <ProductItem key={p.id} product={p} />
            ))}
          </ul>
        ))}

      <AddProductForm
        onAdded={(product) => {
          setList((l) => ({ status: 'ready', products: [product, ...(l.status === 'ready' ? l.products : [])] }))
        }}
      />
    </section>
  )
}

function ProductItem({ product }: { product: Product }) {
  return (
    <li className="product">
      {product.imageUrl && (
        <img src={product.imageUrl} alt={product.name} loading="lazy" referrerPolicy="no-referrer" />
      )}
      <div>
        <h3>{product.name}</h3>
        <p>{formatRupiah(product.priceSen)}</p>
        <p className="meta">{product.stock === 0 ? 'Out of stock' : `${product.stock} in stock`}</p>
        {product.description && <p className="meta">{product.description}</p>}
      </div>
    </li>
  )
}

function AddProductForm({ onAdded }: { onAdded: (product: Product) => void }) {
  const [values, setValues] = useState<FormValues>(emptyForm)
  // Keyed by the API's field names (priceSen, not price), so its messages land on the right input.
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const set = (name: keyof FormValues) => (e: { target: { value: string } }) => {
    setValues((v) => ({ ...v, [name]: e.target.value }))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setFieldErrors({})
    setFormError('')
    setNotice('')

    // Turn the typed text into sen and a whole number first; nothing is sent while either is unclear.
    const priceSen = parseRupiah(values.price)
    const stock = /^\d+$/.test(values.stock.trim()) ? Number(values.stock.trim()) : null
    const local: Record<string, string> = {}
    if (priceSen === null) {
      local.priceSen = 'Enter a price like 85.000 or 1.000,75.'
    }
    if (stock === null) {
      local.stock = 'Enter stock as a whole number, like 12.'
    }
    if (priceSen === null || stock === null) {
      setFieldErrors(local)
      return
    }

    setSubmitting(true)
    try {
      const body = await apiFetch('/api/products', {
        method: 'POST',
        body: {
          name: values.name,
          description: values.description,
          priceSen,
          stock,
          imageUrl: values.imageUrl.trim() === '' ? null : values.imageUrl.trim(),
        },
      })
      if (!isProduct(body)) {
        setFormError('The server sent an unexpected answer. Please reload the page.')
        return
      }
      onAdded(body)
      setValues(emptyForm)
      setNotice(`Added ${body.name}.`)
    } catch (err) {
      if (err instanceof ApiError && err.code === 'validation_failed') {
        setFieldErrors(err.fields)
      } else {
        setFormError(err instanceof ApiError ? err.message : 'Something went wrong. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <h2>Add a product</h2>
      <form
        noValidate
        onSubmit={(e) => {
          void submit(e)
        }}
      >
        <Field id="name" label="Name" value={values.name} onChange={set('name')} error={fieldErrors.name} />
        <TextAreaField
          id="description"
          label="Description"
          value={values.description}
          onChange={set('description')}
          error={fieldErrors.description}
        />
        <Field
          id="price"
          label="Price"
          inputMode="decimal"
          placeholder="85.000"
          value={values.price}
          onChange={set('price')}
          error={fieldErrors.priceSen}
        />
        <Field
          id="stock"
          label="Stock"
          inputMode="numeric"
          placeholder="12"
          value={values.stock}
          onChange={set('stock')}
          error={fieldErrors.stock}
        />
        <Field
          id="imageUrl"
          label="Image URL"
          type="url"
          placeholder="https://… (optional)"
          value={values.imageUrl}
          onChange={set('imageUrl')}
          error={fieldErrors.imageUrl}
        />

        {formError && <p role="alert">{formError}</p>}
        {notice && <p role="status">{notice}</p>}

        <button type="submit" disabled={submitting}>
          {submitting ? 'Adding…' : 'Add product'}
        </button>
      </form>
    </>
  )
}

function isProduct(v: unknown): v is Product {
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

function isProductList(v: unknown): v is { products: Product[] } {
  return (
    typeof v === 'object' &&
    v !== null &&
    'products' in v &&
    Array.isArray(v.products) &&
    v.products.every(isProduct)
  )
}
