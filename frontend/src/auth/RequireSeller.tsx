import { Navigate, Outlet, useLocation } from 'react-router'

import { useAuth } from './auth'

/**
 * Route wrapper for seller-only pages. Logged-out visitors go to
 * /login?next=<where they were going>, so login can bring them back.
 */
export function RequireSeller() {
  const { state } = useAuth()
  const location = useLocation()

  switch (state.status) {
    case 'loading':
      return <p>Loading…</p>
    case 'error':
      return <p role="alert">{state.message}</p>
    case 'loggedOut': {
      const next = encodeURIComponent(location.pathname + location.search)
      return <Navigate to={`/login?next=${next}`} replace />
    }
    case 'loggedIn':
      return <Outlet />
  }
}
