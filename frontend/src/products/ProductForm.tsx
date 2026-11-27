import { type FormEvent, useState } from 'react'

import { ApiError } from '../api'
import { Field, TextAreaField } from '../components/Field'
import { emptyProductForm, type ProductBody, type ProductFormValues, toProductBody } from './product'

type ProductFormProps = {
  initialValues: ProductFormValues
  submitLabel: string
  busyLabel: string
  /** Clear the form after a successful submit (adding); editing navigates away instead. */
  resetAfterSubmit?: boolean
  /**
   * Sends the body. Resolve with a message to show (e.g. "Added Mug."), or
   * throw an ApiError: validation_failed shows per field, anything else as an alert.
   */
  onSubmit: (body: ProductBody) => Promise<string | undefined>
}

/** The fields of a product, used to add one and to edit one. */
export function ProductForm({ initialValues, submitLabel, busyLabel, resetAfterSubmit, onSubmit }: ProductFormProps) {
  const [values, setValues] = useState(initialValues)
  // Keyed by the API's field names (priceSen, not price), so its messages land on the right input.
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState('')
  const [notice, setNotice] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const set = (name: keyof ProductFormValues) => (e: { target: { value: string } }) => {
    setValues((v) => ({ ...v, [name]: e.target.value }))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setFieldErrors({})
    setFormError('')
    setNotice('')

    const parsed = toProductBody(values)
    if ('errors' in parsed) {
      setFieldErrors(parsed.errors)
      return
    }

    setSubmitting(true)
    try {
      const message = await onSubmit(parsed.body)
      if (resetAfterSubmit) {
        setValues(emptyProductForm)
      }
      if (message) {
        setNotice(message)
      }
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
        {submitting ? busyLabel : submitLabel}
      </button>
    </form>
  )
}
