import { createContext, useContext } from 'react'

/** The logged-in seller, as GET /api/me returns them. */
export type Seller = { id: number; email: string; shopName: string }

export type AuthState =
  | { status: 'loading' }
  | { status: 'loggedIn'; seller: Seller }
  | { status: 'loggedOut' }
  | { status: 'error'; message: string }

export type Auth = {
  state: AuthState
  /** After login or signup pass the seller; after logout pass null. */
  setSeller: (seller: Seller | null) => void
  /** Ask the API again who is logged in. */
  refresh: () => void
}

export const AuthContext = createContext<Auth | null>(null)

/** The login state. Only works inside <AuthProvider>. */
export function useAuth(): Auth {
  const auth = useContext(AuthContext)
  if (!auth) {
    throw new Error('useAuth must be used inside <AuthProvider>')
  }
  return auth
}

export function isSeller(v: unknown): v is Seller {
  return (
    typeof v === 'object' &&
    v !== null &&
    'id' in v &&
    typeof v.id === 'number' &&
    'email' in v &&
    typeof v.email === 'string' &&
    'shopName' in v &&
    typeof v.shopName === 'string'
  )
}
