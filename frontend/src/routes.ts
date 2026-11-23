import type { RouteObject } from 'react-router'

import { HomePage } from './pages/HomePage'
import { NotFoundPage } from './pages/NotFoundPage'

/** Every page in the app. main.tsx uses a browser router; tests use a memory router. */
export const routes: RouteObject[] = [
  { path: '/', Component: HomePage },
  { path: '*', Component: NotFoundPage },
]
