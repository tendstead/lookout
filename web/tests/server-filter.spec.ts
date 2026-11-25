import { expect, test } from '@playwright/test';

for (const path of ['/', '/releases']) {
  test(`the server filter follows the theme on ${path}`, async ({ page }) => {
    await page.addInitScript(() =>
      localStorage.setItem('lookout.theme', 'dark'),
    );
    await page.route('**/api/v1/servers', (route) =>
      route.fulfill({ json: { items: [{ id: 'home', name: 'Home NAS' }] } }),
    );
    await page.goto(path);

    const filter = page.getByRole('combobox', { name: 'Servers', exact: true });
    await expect(filter).toBeVisible();
    await expect(filter).toHaveCSS('background-color', 'rgb(65, 54, 83)');
    await expect(filter).toHaveCSS('color', 'rgb(233, 221, 255)');
    await filter.selectOption('home');
    await expect(filter).toHaveValue('home');

    await page.getByRole('button', { name: 'Use light theme' }).click();
    await expect(filter).toHaveCSS('background-color', 'rgb(233, 226, 245)');
    await expect(filter).toHaveCSS('color', 'rgb(72, 51, 117)');
  });
}
