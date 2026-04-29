// Run with both servers up: make editor + make web
import { test, expect } from '@playwright/test'

test.describe('Undo / Redo', () => {
  test('Cmd/Ctrl+Z reverts a context menu edit; Shift+Z redoes', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('combobox').first().click()
    await page.getByRole('option', { name: 'orc' }).click()
    await page.getByRole('button', { name: /^run/ }).click()
    await expect(page.locator('.react-flow__node').first()).toBeVisible({ timeout: 10_000 })

    const node = page.locator('.react-flow__node-action').first()
    await node.click({ button: 'right' })
    await page.getByRole('menuitem', { name: 'Convert to' }).hover()
    await page.getByRole('menuitem', { name: 'condition' }).click()
    await expect(page.locator('.react-flow__node-condition').first()).toBeVisible()

    // Use ControlOrMeta — Playwright maps to Meta on macOS, Control elsewhere.
    await page.keyboard.press('ControlOrMeta+z')
    await expect(page.locator('.react-flow__node-action').first()).toBeVisible()

    await page.keyboard.press('ControlOrMeta+Shift+z')
    await expect(page.locator('.react-flow__node-condition').first()).toBeVisible()
  })

  test('TopBar Undo button enables after edit', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('combobox').first().click()
    await page.getByRole('option', { name: 'orc' }).click()
    await page.getByRole('button', { name: /^run/ }).click()
    await expect(page.locator('.react-flow__node').first()).toBeVisible({ timeout: 10_000 })

    const undoBtn = page.getByRole('button', { name: /^Undo/ })
    await expect(undoBtn).toBeDisabled()

    const node = page.locator('.react-flow__node-action').first()
    await node.click({ button: 'right' })
    await page.getByRole('menuitem', { name: 'Convert to' }).hover()
    await page.getByRole('menuitem', { name: 'condition' }).click()

    await expect(undoBtn).toBeEnabled()
  })
})
