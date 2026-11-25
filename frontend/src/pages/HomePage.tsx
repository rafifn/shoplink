import { useEffect, useState } from 'react'

import { HEALTH_TIMEOUT_MS, isApiHealthy } from '../api'

type ApiStatus = 'checking' | 'ok' | 'unreachable'

const label: Record<ApiStatus, string> = {
  checking: 'Checking API…',
  ok: 'API OK',
  unreachable: 'API unreachable',
}

/** Placeholder home page: shows whether the Go API and its database are up. */
export function HomePage() {
  const [status, setStatus] = useState<ApiStatus>('checking')

  useEffect(() => {
    const controller = new AbortController()
    const timer = setTimeout(() => {
      controller.abort()
    }, HEALTH_TIMEOUT_MS)
    let current = true // false once this effect is cleaned up (StrictMode runs it twice in dev)

    void isApiHealthy(controller.signal)
      .then((ok) => {
        if (current) {
          setStatus(ok ? 'ok' : 'unreachable')
        }
      })
      .finally(() => {
        clearTimeout(timer)
      })

    return () => {
      current = false
      clearTimeout(timer)
      controller.abort()
    }
  }, [])

  return (
    <section>
      <h1>ShopLink</h1>
      <p role="status">{label[status]}</p>
    </section>
  )
}
