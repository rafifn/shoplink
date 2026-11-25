import { type FormEvent, useState } from 'react'
import { Link, Navigate } from 'react-router'

import { ApiError, apiFetch } from '../api'
import { isSeller, useAuth } from '../auth/auth'
import { Field } from '../components/Field'

type Values = { email: string; password: string; shopName: string }

/** Create a seller account. Success logs the seller in and goes to /products. */
export function SignupPage() {
  const { state, setSeller } = useAuth()
  const [values, setValues] = useState<Values>({ email: '', password: '', shopName: '' })
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [emailTaken, setEmailTaken] = useState(false)
  const [formError, setFormError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  // Logged in already, or just signed up: setSeller() below lands here too.
  if (state.status === 'loggedIn') {
    return <Navigate to="/products" replace />
  }

  const set = (name: keyof Values) => (e: { target: { value: string } }) => {
    setValues((v) => ({ ...v, [name]: e.target.value }))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setFieldErrors({})
    setEmailTaken(false)
    setFormError('')
    try {
      const body = await apiFetch('/api/auth/signup', { method: 'POST', body: values })
      if (isSeller(body)) {
        setSeller(body)
      } else {
        setFormError('The server sent an unexpected answer. Please try again.')
      }
    } catch (err) {
      if (err instanceof ApiError && err.code === 'validation_failed') {
        setFieldErrors(err.fields)
      } else if (err instanceof ApiError && err.code === 'email_taken') {
        setFieldErrors({ email: err.message })
        setEmailTaken(true)
      } else {
        setFormError(err instanceof ApiError ? err.message : 'Something went wrong. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section>
      <h1>Sign up</h1>
      <p>Create a seller account to add products and share checkout links.</p>

      {/* noValidate: the API checks every rule and its messages show per field. */}
      <form
        noValidate
        onSubmit={(e) => {
          void submit(e)
        }}
      >
        <Field
          id="email"
          label="Email"
          type="email"
          autoComplete="email"
          value={values.email}
          onChange={set('email')}
          error={fieldErrors.email}
        />
        {emailTaken && (
          <p>
            <Link to="/login">Log in with that email</Link> instead.
          </p>
        )}
        <Field
          id="password"
          label="Password"
          type="password"
          autoComplete="new-password"
          value={values.password}
          onChange={set('password')}
          error={fieldErrors.password}
        />
        <Field
          id="shopName"
          label="Shop name"
          autoComplete="organization"
          value={values.shopName}
          onChange={set('shopName')}
          error={fieldErrors.shopName}
        />

        {formError && <p role="alert">{formError}</p>}

        <button type="submit" disabled={submitting}>
          {submitting ? 'Creating account…' : 'Create account'}
        </button>
      </form>

      <p>
        Already have an account? <Link to="/login">Log in</Link>
      </p>
    </section>
  )
}
