import { defineConfig, devices } from '@playwright/test'

// Use the repo's .env (DATABASE_URL, PORT, FRONTEND_ORIGIN) so `npx playwright
// test` works on its own too. Values already set, e.g. by `make e2e`, win.
try {
  process.loadEnvFile(new URL('../.env', import.meta.url))
} catch {
  // No .env (`make e2e` creates it from .env.example); rely on the environment.
}

const frontendURL = process.env.FRONTEND_ORIGIN || 'http://localhost:5173'
const apiURL = `http://localhost:${process.env.PORT || '8080'}`
// Locally, reuse servers that are already up (e.g. from `make dev`); CI always starts fresh ones.
const reuse = !process.env.CI

export default defineConfig({
  testDir: './tests',
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: frontendURL,
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],

  // Postgres isn't started here: `make e2e` runs `make db-up` first. The API is
  // only counted as up once /healthz answers 200, i.e. it can reach the database.
  webServer: [
    {
      name: 'api',
      cwd: '../backend',
      command: 'go build -o bin/api ./cmd/api && exec ./bin/api',
      url: `${apiURL}/healthz`,
      reuseExistingServer: reuse,
      timeout: 120_000, // the first build can be slow
      gracefulShutdown: { signal: 'SIGTERM', timeout: 10_000 },
    },
    {
      name: 'frontend',
      cwd: '../frontend',
      command: 'exec ./node_modules/.bin/vite',
      url: frontendURL,
      reuseExistingServer: reuse,
      timeout: 60_000,
      gracefulShutdown: { signal: 'SIGTERM', timeout: 5_000 },
    },
  ],
})
