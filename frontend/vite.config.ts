/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  // Read the repo's single .env. Only VITE_-prefixed values (VITE_API_URL)
  // reach the browser bundle; the database passwords never do.
  envDir: '..',
  server: {
    port: 5173,
    // Fail instead of moving to another port: the API's CORS allows exactly
    // FRONTEND_ORIGIN (http://localhost:5173).
    strictPort: true,
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    unstubEnvs: true,
    unstubGlobals: true,
  },
})
