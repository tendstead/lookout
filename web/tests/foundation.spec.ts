import { test, expect } from '@playwright/test';
test('foundation is usable, bilingual and persists preferences', async ({
  page,
}, testInfo) => {
  await page.goto('/');
  await expect(
    page.getByRole('heading', { name: 'Your media, in view.' }),
  ).toBeVisible();
  await expect(page.getByTestId('database-status')).toHaveText('Connected');
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.getByRole('button', { name: 'DE', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Deine Medien im Blick.' }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole('heading', { name: 'Deine Medien im Blick.' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'EN', exact: true }).click();
  await page.getByRole('button', { name: 'Use dark theme' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.getByRole('button', { name: 'Use light theme' }).click();
  await expect(page.getByTestId('database-status')).toHaveText('Connected');
  if (process.env.UPDATE_SCREENSHOTS === '1') {
    if (testInfo.project.name === 'mobile') {
      await page.goto('/releases');
      await expect(
        page.getByRole('heading', {
          name: 'Seasons missing from your library',
          exact: true,
        }),
      ).toBeVisible();
      const gaps = page.getByRole('region', {
        name: 'Seasons missing from your library',
        exact: true,
      });
      await expect(gaps.getByRole('status')).toHaveCount(0);
      await gaps.evaluate((el) => el.scrollIntoView({ block: 'start' }));
    }
    await page.screenshot({
      path: `../docs/screenshots/${testInfo.project.name}.png`,
      fullPage: false,
      scale: 'css',
    });
  }
});
test('unavailable API leaves the shell usable', async ({ page }) => {
  await page.route('**/readyz', (route) =>
    route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: '{"status":"unavailable"}',
    }),
  );
  await page.goto('/');
  await expect(page.getByTestId('database-status')).toHaveText('Unavailable');
  await expect(
    page.getByRole('button', { name: 'DE', exact: true }),
  ).toBeVisible();
});
test('API never returns the SPA', async ({ request }) => {
  const r = await request.get('/api/v1/does-not-exist');
  expect(r.status()).toBe(404);
  expect(r.headers()['content-type']).toContain('application/json');
  expect((await r.json()).error.code).toBe('not_found');
});

test('shell controls and navigation follow the selected language', async ({
  page,
}, info) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'DE', exact: true }).click();
  if (info.project.name === 'mobile') {
    await page.getByRole('button', { name: 'Menü', exact: true }).click();
    await expect(
      page.getByRole('button', { name: 'Menü schließen' }),
    ).toBeVisible();
  }
  await page
    .getByRole('link', { name: 'System', exact: true })
    .filter({ visible: true })
    .click();
  await expect(
    page.getByRole('heading', { name: 'Dienststatus', exact: true }).first(),
  ).toBeVisible();
  await page.reload();
  await expect(page).toHaveURL(/\/system$/);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});
