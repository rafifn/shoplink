import { expect, test } from '@playwright/test'

// The whole stack is wired up: the browser loads the React app, which calls the
// Go API across origins (CORS), which pings Postgres.
test('home page shows "API OK"', async ({ page }) => {
  await page.goto('/')

  await expect(page).toHaveTitle('ShopLink')
  await expect(page.getByRole('status')).toHaveText('API OK')
})
