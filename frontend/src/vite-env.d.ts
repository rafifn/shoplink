// Build-time settings from the repo's .env (see vite.config.ts and .env.example).
interface ImportMetaEnv {
  /** The API's origin, e.g. http://localhost:8080. Empty when the API is on the same domain. */
  readonly VITE_API_URL?: string
}
