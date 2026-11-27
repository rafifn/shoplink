import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'

import { ApiError, apiFetch } from '../api'
import { formValuesFrom, isProduct, type Product } from '../products/product'
import { ProductForm } from '../products/ProductForm'

type PageState =
  | { status: 'loading' }
  | { status: 'notFound' }
  | { status: 'error'; message: string }
  | { status: 'ready'; product: Product }

/** Edit one product, then go back to the list. */
export function EditProductPage() {
  const params = useParams()
  // Anything but a positive whole number can't be a product: no need to ask the API.
  const id = params.id && /^[1-9]\d*$/.test(params.id) ? params.id : null
  const navigate = useNavigate()
  const [page, setPage] = useState<PageState>(id ? { status: 'loading' } : { status: 'notFound' })

  useEffect(() => {
    if (!id) {
      return
    }
    const controller = new AbortController()
    apiFetch(`/api/products/${id}`, { signal: controller.signal })
      .then((body) => {
        if (!controller.signal.aborted) {
          setPage(
            isProduct(body)
              ? { status: 'ready', product: body }
              : { status: 'error', message: 'The server sent an unexpected answer. Please reload the page.' },
          )
        }
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) {
          return
        }
        // 404 covers another seller's product too; the API doesn't say which.
        if (err instanceof ApiError && err.status === 404) {
          setPage({ status: 'notFound' })
        } else {
          setPage({ status: 'error', message: err instanceof ApiError ? err.message : 'Could not load this product.' })
        }
      })
    return () => {
      controller.abort()
    }
  }, [id])

  switch (page.status) {
    case 'loading':
      return <p>Loading the product…</p>
    case 'error':
      return <p role="alert">{page.message}</p>
    case 'notFound':
      return (
        <section>
          <h1>Product not found</h1>
          <p>It may have been archived, or the link is wrong.</p>
          <p>
            <Link to="/products">Back to your products</Link>
          </p>
        </section>
      )
    case 'ready':
      return (
        <section>
          <h1>Edit {page.product.name}</h1>
          <ProductForm
            key={page.product.id}
            initialValues={formValuesFrom(page.product)}
            submitLabel="Save changes"
            busyLabel="Saving…"
            onSubmit={async (body) => {
              const saved = await apiFetch(`/api/products/${page.product.id}`, { method: 'PATCH', body })
              if (!isProduct(saved)) {
                throw new ApiError(200, 'bad_response', 'The server sent an unexpected answer. Please reload the page.')
              }
              await navigate('/products', { state: { notice: `Saved ${saved.name}.` } })
              return undefined
            }}
          />
          <p>
            <Link to="/products">Cancel</Link>
          </p>
        </section>
      )
  }
}
