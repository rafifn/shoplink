import { useState } from 'react'
import { Link, Outlet, useNavigate } from 'react-router'

import { apiFetch } from './api'
import { AuthProvider } from './auth/AuthProvider'
import { useAuth } from './auth/auth'

/** The root of every page: login state, the header, and the page itself. */
export function App() {
  return (
    <AuthProvider>
      <Layout />
    </AuthProvider>
  )
}

function Layout() {
  const { state } = useAuth()

  return (
    <>
      <header className="site-header">
        <Link to="/" className="brand">
          ShopLink
        </Link>
        <nav aria-label="Main" className="site-nav">
          {state.status === 'loggedIn' && (
            <>
              <Link to="/products">Products</Link>
              <span className="shop-name">{state.seller.shopName}</span>
              <LogoutButton />
            </>
          )}
          {state.status === 'loggedOut' && (
            <>
              <Link to="/login">Log in</Link>
              <Link to="/signup">Sign up</Link>
            </>
          )}
        </nav>
      </header>
      <main className="page">
        <Outlet />
      </main>
    </>
  )
}

/** Ends the session, then shows the logged-out home page. On failure the seller stays logged in. */
function LogoutButton() {
  const { setSeller } = useAuth()
  const navigate = useNavigate()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  async function logOut() {
    setBusy(true)
    setError('')
    try {
      await apiFetch('/api/auth/logout', { method: 'POST' })
      // Leave the seller-only page before clearing the login state; otherwise
      // it re-renders logged out and redirects to /login. flushSync applies the
      // route change right away instead of as a low-priority transition.
      await navigate('/', { flushSync: true })
      setSeller(null)
    } catch {
      setError("Couldn't log out. Please try again.")
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <button
        type="button"
        className="link-button"
        disabled={busy}
        onClick={() => {
          void logOut()
        }}
      >
        Log out
      </button>
      {error && <span role="alert">{error}</span>}
    </>
  )
}
