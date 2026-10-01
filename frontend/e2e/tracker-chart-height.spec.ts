import { test, expect } from '@playwright/test'

const CHART_FRAME = '.aspect-\\[2\\/1\\]'

async function signIn(page: any): Promise<void> {
  await page.goto('/')
  await page.getByText('Login').click()
  await page.getByRole('button', { name: 'Sign In' }).click()
  await expect(page.locator('header .bg-blue-500')).toBeVisible({ timeout: 10_000 })
}

async function firstMetricTrackerId(page: any): Promise<number> {
  return page.evaluate(async () => {
    const res = await fetch('/api/trackers?page=1&per_page=100')
    const body: any = await res.json()
    const trackers = body.trackers ?? body.data ?? []
    const found = trackers.find((t: any) => t.type !== 'coverage')
    return found ? found.id : 0
  })
}

test.describe('Tracker chart sizing', () => {
  test('clamps the canvas to the mobile floor so the plot is not portrait', async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 640 })
    await signIn(page)

    const trackerId = await firstMetricTrackerId(page)
    expect(trackerId).toBeGreaterThan(0)

    await page.goto(`/trackers/${trackerId}`)
    const frame = page.locator(CHART_FRAME).first()
    await frame.waitFor({ timeout: 15_000 })
    await expect(frame.locator('svg').first()).toBeVisible()

    const box = (await frame.boundingBox())!
    expect(box.height).toBeGreaterThanOrEqual(200)
    expect(box.height).toBeLessThanOrEqual(201)
    // The canvas must be filled: height:100% resolves against the frame.
    const svg = (await frame.locator('svg').first().boundingBox())!
    expect(svg.height).toBeGreaterThan(0)
    expect(box.width).toBeGreaterThan(box.height)
  })

  test('caps the canvas at the desktop maximum', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await signIn(page)

    const trackerId = await firstMetricTrackerId(page)
    expect(trackerId).toBeGreaterThan(0)

    await page.goto(`/trackers/${trackerId}`)
    const frame = page.locator(CHART_FRAME).first()
    await frame.waitFor({ timeout: 15_000 })
    await expect(frame.locator('svg').first()).toBeVisible()

    const box = (await frame.boundingBox())!
    expect(box.height).toBeGreaterThanOrEqual(300)
    expect(box.height).toBeLessThanOrEqual(301)
  })
})