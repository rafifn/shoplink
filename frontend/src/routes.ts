import type { RouteObject } from 'react-router'

import { RequireSeller } from './auth/RequireSeller'
import { App } from './Layout'
import { HomePage } from './pages/HomePage'
import { LoginPage } from './pages/LoginPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { ProductsPage } from './pages/ProductsPage'
import { SignupPage } from './pages/SignupPage'

/** Every page in the app. main.tsx uses a browser router; tests use a memory router. */
export const routes: RouteObject[] = [
  {
    Component: App,
    children: [
      { path: '/', Component: HomePage },
      { path: '/login', Component: LoginPage },
      { path: '/signup', Component: SignupPage },
      // Seller-only pages: logged-out visitors are sent to /login.
      { Component: RequireSeller, children: [{ path: '/products', Component: ProductsPage }] },
      { path: '*', Component: NotFoundPage },
    ],
  },
]
