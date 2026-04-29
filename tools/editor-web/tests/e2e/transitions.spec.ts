// Run with both servers up: make editor + make web
import { test, expect } from '@playwright/test'

test.describe('Transitions tab', () => {
  test('renders state graph and links back to BT tab', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('combobox').first().click()
    await page.getByRole('option', { name: 'orc' }).click()

    await page.getByRole('tab', { name: 'Transitions' }).click()

    // orc states: fall, run, attack, attack2, hurt, death
    for (const id of ['fall', 'run', 'attack', 'attack2', 'hurt', 'death']) {
      await expect(page.locator(`.react-flow__node[data-id="${id}"]`)).toBeVisible({ timeout: 10_000 })
    }

    await page.locator('.react-flow__node[data-id="attack"]').click()
    await expect(page.getByRole('tab', { name: 'BT' })).toHaveAttribute('data-state', 'active')
  })
})
