import { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'

import { ApiError, apiFetch } from '../api'
import { formatRupiah } from '../money'
import { emptyProductForm, isProduct, isProductList, type Product } from '../products/product'
import { ProductForm } from '../products/ProductForm'

type ListState = { status: 'loading' } | { status: 'error'; message: string } | { status: 'ready'; products: Product[] }

/** The seller's products, newest first, with a form to add one. */
export function ProductsPage() {
  const [list, setList] = useState<ListState>({ status: 'loading' })
  const flash = useFlashNotice()

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
      {flash && <p role="status">{flash}</p>}

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

      <h2>Add a product</h2>
      <ProductForm
        initialValues={emptyProductForm}
        submitLabel="Add product"
        busyLabel="Adding…"
        resetAfterSubmit
        onSubmit={async (body) => {
          const created = await apiFetch('/api/products', { method: 'POST', body })
          if (!isProduct(created)) {
            throw new ApiError(200, 'bad_response', 'The server sent an unexpected answer. Please reload the page.')
          }
          setList((l) => ({ status: 'ready', products: [created, ...(l.status === 'ready' ? l.products : [])] }))
          return `Added ${created.name}.`
        }}
      />
    </section>
  )
}

function ProductItem({ product }: { product: Product }) {
  return (
    <li className="product">
      {product.imageUrl && <img src={product.imageUrl} alt={product.name} loading="lazy" referrerPolicy="no-referrer" />}
      <div>
        <h3>{product.name}</h3>
        <p>{formatRupiah(product.priceSen)}</p>
        <p className="meta">{product.stock === 0 ? 'Out of stock' : `${product.stock} in stock`}</p>
        {product.description && <p className="meta">{product.description}</p>}
        <p>
          <Link to={`/products/${product.id}/edit`} aria-label={`Edit ${product.name}`}>
            Edit
          </Link>
        </p>
      </div>
    </li>
  )
}

/**
 * A one-time message passed in navigation state, e.g. "Saved Mug." after an
 * edit. The state is cleared right away, so a reload doesn't show it again.
 */
function useFlashNotice(): string {
  const location = useLocation()
  const navigate = useNavigate()
  const [notice] = useState(() => {
    const state: unknown = location.state
    return typeof state === 'object' && state !== null && 'notice' in state && typeof state.notice === 'string'
      ? state.notice
      : ''
  })
  useEffect(() => {
    if (notice) {
      void navigate(location.pathname, { replace: true, state: null })
    }
  }, [notice, navigate, location.pathname])
  return notice
}
