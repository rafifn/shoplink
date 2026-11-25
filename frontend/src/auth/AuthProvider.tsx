import { type ReactNode, useCallback, useEffect, useMemo, useState } from 'react'

import { ApiError, apiFetch } from '../api'
import { type AuthState, AuthContext, isSeller, type Seller } from './auth'

/** Asks GET /api/me who is logged in and shares the answer with every page. */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' })
  const [checks, setChecks] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    apiFetch('/api/me', { signal: controller.signal })
      .then((body) => {
        if (controller.signal.aborted) {
          return
        }
        setState(
          isSeller(body)
            ? { status: 'loggedIn', seller: body }
            : { status: 'error', message: 'The server sent an unexpected answer. Please reload the page.' },
        )
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) {
          return // StrictMode or a newer check replaced this one
        }
        if (err instanceof ApiError && err.status === 401) {
          setState({ status: 'loggedOut' })
        } else {
          setState({
            status: 'error',
            message: err instanceof ApiError ? err.message : 'Something went wrong. Please reload the page.',
          })
        }
      })
    return () => {
      controller.abort()
    }
  }, [checks])

  const setSeller = useCallback((seller: Seller | null) => {
    setState(seller ? { status: 'loggedIn', seller } : { status: 'loggedOut' })
  }, [])
  const refresh = useCallback(() => {
    setChecks((n) => n + 1)
  }, [])
  const auth = useMemo(() => ({ state, setSeller, refresh }), [state, setSeller, refresh])

  return <AuthContext value={auth}>{children}</AuthContext>
}
