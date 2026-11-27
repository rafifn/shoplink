/** Where a seller lands after logging in when there's no usable ?next. */
export const DEFAULT_AFTER_LOGIN = '/products'

/**
 * The page to go to after logging in. Only a path on this site counts:
 * "https://evil.example.com", "//evil.example.com" and "/\evil.example.com"
 * (browsers read the last two as another site) fall back to /products, so a
 * crafted link can't use the login page to send people elsewhere.
 */
export function safeNext(next: string | null): string {
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) {
    return DEFAULT_AFTER_LOGIN
  }
  return next
}
