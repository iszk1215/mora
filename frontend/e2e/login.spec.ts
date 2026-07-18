import { test, expect } from '@playwright/test'

test.describe('Login flow', () => {
  test('shows Login link when not authenticated', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByText('Login')).toBeVisible()
  })

  test('displays avatar after login', async ({ page }) => {
    await page.goto('/')

    await page.getByText('Login').click()

    await page.getByLabel('Username').fill('demo')
    await page.getByLabel('Password').fill('demo')
    await page.getByRole('button', { name: 'Sign In' }).click()

    await expect(page.locator('header .bg-blue-500')).toBeVisible({ timeout: 10_000 })
  })
})
