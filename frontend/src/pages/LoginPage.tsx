import { type FormEvent, useState } from 'react'
import { Link, Navigate, useSearchParams } from 'react-router'

import { ApiError, apiFetch } from '../api'
import { isSeller, useAuth } from '../auth/auth'
import { safeNext } from '../auth/safeNext'
import { Field } from '../components/Field'

type Values = { email: string; password: string }

/** Log in, then go back to ?next (a page on this site) or to /products. */
export function LoginPage() {
  const { state, setSeller } = useAuth()
  const [searchParams] = useSearchParams()
  const next = safeNext(searchParams.get('next'))
  const [values, setValues] = useState<Values>({ email: '', password: '' })
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  // Logged in already, or just logged in: setSeller() below lands here too.
  if (state.status === 'loggedIn') {
    return <Navigate to={next} replace />
  }

  const set = (name: keyof Values) => (e: { target: { value: string } }) => {
    setValues((v) => ({ ...v, [name]: e.target.value }))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setFieldErrors({})
    setFormError('')
    try {
      const body = await apiFetch('/api/auth/login', { method: 'POST', body: values })
      if (isSeller(body)) {
        setSeller(body)
      } else {
        setFormError('The server sent an unexpected answer. Please try again.')
      }
    } catch (err) {
      if (err instanceof ApiError && err.code === 'validation_failed') {
        setFieldErrors(err.fields)
      } else {
        // Includes invalid_credentials, which deliberately doesn't say which field was wrong.
        setFormError(err instanceof ApiError ? err.message : 'Something went wrong. Please try again.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section>
      <h1>Log in</h1>

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
        <Field
          id="password"
          label="Password"
          type="password"
          autoComplete="current-password"
          value={values.password}
          onChange={set('password')}
          error={fieldErrors.password}
        />

        {formError && <p role="alert">{formError}</p>}

        <button type="submit" disabled={submitting}>
          {submitting ? 'Logging in…' : 'Log in'}
        </button>
      </form>

      <p>
        New here? <Link to="/signup">Create an account</Link>
      </p>
    </section>
  )
}
