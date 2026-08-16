import { test, expect } from '@playwright/test'

test.describe('Login flow', () => {
  test('shows Login link when not authenticated', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByText('Login')).toBeVisible()
  })

  test('displays avatar after password login', async ({ page }) => {
    await page.goto('/')

    await page.getByText('Login').click()

    await page.getByLabel('Username').fill('demo')
    await page.getByLabel('Password').fill('demo')
    await page.getByRole('button', { name: 'Sign In' }).click()

    await expect(page.locator('header .bg-blue-500')).toBeVisible({ timeout: 10_000 })
  })

  test('displays avatar after OAuth login', async ({ page }) => {
    await page.goto('/auth')

    await page.getByRole('link', { name: /Login with Gitea/ }).click()

    await page.waitForURL('/signup', { timeout: 15_000 })

    await expect(page.getByRole('heading', { name: 'Create Account' })).toBeVisible()
    await page.getByRole('button', { name: 'Create Account' }).click()

    await page.waitForURL('/')
    await expect(page.locator('header img')).toBeVisible({ timeout: 10_000 })
  })

  test('displays avatar after Google login', async ({ page }) => {
    await page.goto('/auth')

    await page.getByRole('link', { name: /Login with Google/ }).click()

    await page.waitForURL('/signup', { timeout: 15_000 })

    await expect(page.getByRole('heading', { name: 'Create Account' })).toBeVisible()
    await expect(page.getByText('First time logging in with google.')).toBeVisible()
    await page.getByRole('button', { name: 'Create Account' }).click()

    await page.waitForURL('/')
    await expect(page.locator('header img')).toBeVisible({ timeout: 10_000 })
  })
})
