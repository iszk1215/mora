import { test, expect } from '@playwright/test'

async function signIn(page: any): Promise<void> {
  await page.goto('/')
  await page.getByText('Login').click()
  await page.getByRole('button', { name: 'Sign In' }).click()
  await expect(page.locator('header .bg-blue-500')).toBeVisible({ timeout: 10_000 })
}

async function ownedTrackerId(page: any): Promise<number> {
  return page.evaluate(async () => {
    const res = await fetch('/api/trackers?page=1&per_page=100')
    const body: any = await res.json()
    const trackers = body.trackers ?? body.data ?? []
    const found = trackers.find((t: any) => t.type !== 'coverage' && t.role === 'owner')
    return found ? found.id : 0
  })
}

async function valueRowBoxes(page: any) {
  const row = page.getByRole('group', { name: 'Add value' })
  await row.waitFor({ timeout: 15_000 })
  const rowBox = await row.boundingBox()
  const date = await row.getByLabel('Value date').boundingBox()
  const series = await row.getByRole('button', { name: 'Series' }).boundingBox()
  const value = await row.getByRole('spinbutton', { name: 'Value', exact: true }).boundingBox()
  const add = await row.getByRole('button', { name: 'Add' }).boundingBox()
  return { row: rowBox!, date: date!, series: series!, value: value!, add: add! }
}

test.describe('Quick value input layout', () => {
  test('keeps every control on a single row on desktop', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 })
    await signIn(page)

    const trackerId = await ownedTrackerId(page)
    expect(trackerId).toBeGreaterThan(0)

    await page.goto(`/trackers/${trackerId}`)
    const { date, series, value, add } = await valueRowBoxes(page)

    expect(Math.abs(series.y - date.y)).toBeLessThan(4)
    expect(Math.abs(value.y - date.y)).toBeLessThan(4)
    expect(Math.abs(add.y - date.y)).toBeLessThan(4)
    expect(value.x).toBeGreaterThan(series.x + series.width - 1)
  })

  test('stacks the date control and the series/value controls on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 360, height: 640 })
    await signIn(page)

    const trackerId = await ownedTrackerId(page)
    expect(trackerId).toBeGreaterThan(0)

    await page.goto(`/trackers/${trackerId}`)
    const { row, date, series, value, add } = await valueRowBoxes(page)

    // Row 1: the date control. Row 2: series, value and Add.
    expect(series.y).toBeGreaterThan(date.y + date.height - 1)
    expect(Math.abs(value.y - series.y)).toBeLessThan(4)
    expect(Math.abs(add.y - series.y)).toBeLessThan(4)
    // The series dropdown keeps more than half of the second row.
    expect(series.width).toBeGreaterThan(row.width / 2)
    expect(value.x).toBeGreaterThan(series.x + series.width - 1)
  })
})