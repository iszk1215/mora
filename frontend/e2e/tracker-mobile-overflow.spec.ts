import { test, expect } from '@playwright/test'

async function signIn(page: any): Promise<void> {
  await page.goto('/')
  await page.getByText('Login').click()
  await page.getByRole('button', { name: 'Sign In' }).click()
  await expect(page.locator('header .bg-blue-500')).toBeVisible({ timeout: 10_000 })
}

async function ownedTracker(page: any): Promise<{ id: number; name: string }> {
  return page.evaluate(async () => {
    const res = await fetch('/api/trackers?page=1&per_page=100')
    const body: any = await res.json()
    const trackers = body.trackers ?? body.data ?? []
    const found = trackers.find((t: any) => t.type !== 'coverage' && t.role === 'owner')
    return found ? { id: found.id, name: found.name } : { id: 0, name: '' }
  })
}

async function noHorizontalOverflow(page: any): Promise<void> {
  const { scrollWidth, innerWidth } = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    innerWidth: window.innerWidth,
  }))
  expect(scrollWidth).toBeLessThanOrEqual(innerWidth + 1)
}

test.describe('Tracker detail mobile layout', () => {
  test('cards do not overflow horizontally at 360px', async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 640 })
    await signIn(page)

    const tracker = await ownedTracker(page)
    expect(tracker.id).toBeGreaterThan(0)

    await page.goto(`/trackers/${tracker.id}`)
    await expect(page.getByRole('heading', { level: 1, name: tracker.name })).toBeVisible({ timeout: 15_000 })
    await noHorizontalOverflow(page)

    for (const label of ['Series', 'Data Points', 'Settings']) {
      await page.goto(`/trackers/${tracker.id}`)
      await expect(page.getByRole('button', { name: 'Tracker menu' })).toBeVisible({ timeout: 15_000 })
      await page.getByRole('button', { name: 'Tracker menu' }).click()
      await page.getByRole('menuitem', { name: label }).click()
      await noHorizontalOverflow(page)
    }
  })
})