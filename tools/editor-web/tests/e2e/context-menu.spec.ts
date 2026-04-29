// Run with both servers up: make editor + make web
import { test, expect } from '@playwright/test'

async function openOrcRun(page: import('@playwright/test').Page) {
  await page.goto('/')
  await page.getByRole('combobox').first().click()
  await page.getByRole('option', { name: 'orc' }).click()
  await page.getByRole('button', { name: /^run/ }).click()
  await expect(page.locator('.react-flow__node').first()).toBeVisible({ timeout: 10_000 })
}

test.describe('Context menu', () => {
  test('right-click action node converts to condition', async ({ page }) => {
    await openOrcRun(page)
    const node = page.locator('.react-flow__node-action').first()
    await expect(node).toBeVisible()
    await node.click({ button: 'right' })
    await page.getByRole('menuitem', { name: 'Convert to' }).hover()
    await page.getByRole('menuitem', { name: 'condition' }).click()
    await expect(page.locator('.react-flow__node-condition').first()).toBeVisible()
  })

  test('root node delete is disabled', async ({ page }) => {
    await openOrcRun(page)
    const root = page.locator('.react-flow__node').first()
    await root.click({ button: 'right' })
    const del = page.getByRole('menuitem', { name: 'Delete' })
    await expect(del).toHaveAttribute('aria-disabled', 'true')
  })
})
