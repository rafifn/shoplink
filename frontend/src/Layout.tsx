import { Link, Outlet } from 'react-router'

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
              {/* The logout button joins the shop name in #32. */}
              <span className="shop-name">{state.seller.shopName}</span>
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
