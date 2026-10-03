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

test.describe('Tracker title font size', () => {
  test('shrinks the tracker name on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 640 })
    await signIn(page)

    const tracker = await ownedTracker(page)
    expect(tracker.id).toBeGreaterThan(0)

    await page.goto(`/trackers/${tracker.id}`)
    await expect(page.getByRole('heading', { level: 1, name: tracker.name })).toHaveCSS('font-size', '24px', { timeout: 15_000 })
  })

  test('keeps the desktop tracker name at text-3xl', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await signIn(page)

    const tracker = await ownedTracker(page)
    expect(tracker.id).toBeGreaterThan(0)

    await page.goto(`/trackers/${tracker.id}`)
    await expect(page.getByRole('heading', { level: 1, name: tracker.name })).toHaveCSS('font-size', '30px', { timeout: 15_000 })
  })
})