// Run with both servers up: make editor + make web
import { test, expect } from '@playwright/test'

test.describe('Behavior editor', () => {
  test('loads orc + renders BT for run state', async ({ page }) => {
    await page.goto('/')

    // Pick orc via TopBar Select. Radix Select needs click to open.
    await page.getByRole('combobox').first().click()
    await page.getByRole('option', { name: 'orc' }).click()

    // States panel: 6 buttons including 'run' with BT badge
    await expect(page.getByRole('button', { name: /^run/ })).toBeVisible()
    await page.getByRole('button', { name: /^run/ }).click()

    // BT canvas: at least one custom node visible
    await expect(page.locator('.react-flow__node').first()).toBeVisible({ timeout: 10_000 })
  })

  test('switches between hand and select mode', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('combobox').first().click()
    await page.getByRole('option', { name: 'orc' }).click()
    await page.getByRole('button', { name: /^run/ }).click()

    // ToggleGroup at top-right of canvas
    await page.getByRole('radio', { name: /select/i }).click()
    await expect(page.getByRole('radio', { name: /select/i })).toHaveAttribute('aria-checked', 'true')
  })
})
